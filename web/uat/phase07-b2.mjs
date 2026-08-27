// Phase 07 UAT B2 自动化证据脚本（Linux 侧运行）
// SEC-07 多值头/空值头真实服务行为：原始 socket 手构 WS Upgrade（两行同名头/空值头），
// 无效 ticket 触发 auth_failed close 事件行，断言 remote_user 取首值 / 空值不出键。
// 红线：凭据值只作请求构造材料，永不进输出。
import { spawn } from 'node:child_process';
import net from 'node:net';
import crypto from 'node:crypto';

const WESH = '/tmp/wesh-uat/wesh';
const CRED = 'uat:uat-pass-x9'; // UAT 专用一次性凭据（phase07.mjs 同款；值不入输出）
const enc = new TextEncoder();

const results = [];
const check = (id, name, ok, detail = '') => {
  results.push(ok);
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};

function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, args, { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '', stdoutBuf = '';
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error('启动超时')); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      stdoutBuf += d.toString();
      const m = /listening on https?:\/\/[^\s]+:(\d+)/.exec(stdoutBuf);
      if (m) { clearTimeout(to); setTimeout(() => resolve({ port: +m[1], child, stderrText: () => stderr }), 50); }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

// 手构 WS Upgrade：rawHeaderLines 原样逐行写入（重复头行的唯一可控通道）
function rawUpgrade(port, path, rawHeaderLines) {
  return new Promise((resolve, reject) => {
    const key = crypto.randomBytes(16).toString('base64');
    const socket = net.createConnection(port, '127.0.0.1');
    let buf = Buffer.alloc(0);
    const to = setTimeout(() => { socket.destroy(); reject(new Error('upgrade 超时')); }, 5000);
    socket.on('connect', () => {
      const lines = [
        `GET ${path} HTTP/1.1`,
        `Host: 127.0.0.1:${port}`,
        'Upgrade: websocket',
        'Connection: Upgrade',
        `Sec-WebSocket-Key: ${key}`,
        'Sec-WebSocket-Version: 13',
        'Sec-WebSocket-Protocol: wesh.v1',
        ...rawHeaderLines,
        '', '',
      ];
      socket.write(lines.join('\r\n'));
    });
    socket.on('data', function onData(d) {
      buf = Buffer.concat([buf, d]);
      const idx = buf.indexOf('\r\n\r\n');
      if (idx < 0) return;
      clearTimeout(to);
      socket.off('data', onData);
      const head = buf.subarray(0, idx).toString();
      const status = +(/HTTP\/1\.1 (\d+)/.exec(head)?.[1] ?? 0);
      resolve({ socket, status, rest: buf.subarray(idx + 4) });
    });
    socket.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

// 客户端→服务端帧（RFC 6455 必须掩码）；payload < 126 短形态
function maskFrame(payload) {
  const mask = crypto.randomBytes(4);
  const frame = Buffer.alloc(2 + 4 + payload.length);
  frame[0] = 0x82; // FIN + binary
  frame[1] = 0x80 | payload.length;
  mask.copy(frame, 2);
  for (let i = 0; i < payload.length; i++) frame[6 + i] = payload[i] ^ mask[i & 3];
  return frame;
}

// 读服务端帧直到 close（或超时）；返回 { code, reason } 或 null
function readUntilClose(socket, firstChunk, timeoutMs = 2500) {
  return new Promise((resolve) => {
    let buf = firstChunk ?? Buffer.alloc(0);
    const to = setTimeout(() => resolve(null), timeoutMs);
    const tryParse = () => {
      while (buf.length >= 2) {
        const opcode = buf[0] & 0x0f;
        let len = buf[1] & 0x7f, off = 2;
        if (len === 126) { if (buf.length < 4) return; len = buf.readUInt16BE(2); off = 4; }
        if (buf.length < off + len) return;
        const payload = buf.subarray(off, off + len);
        buf = buf.subarray(off + len);
        if (opcode === 0x8) {
          clearTimeout(to);
          resolve({ code: payload.length >= 2 ? payload.readUInt16BE(0) : 1005, reason: payload.subarray(2).toString() });
          return;
        }
      }
    };
    socket.on('data', (d) => { buf = Buffer.concat([buf, d]); tryParse(); });
    tryParse();
  });
}

const helloBadTicket = () =>
  Buffer.concat([Buffer.from([0x48]), enc.encode(JSON.stringify({ version: 'wesh.v1', cols: 80, rows: 24, ticket: 'uat-invalid-ticket-000' }))]);

async function probeAuthFailed(port, rawHeaderLines) {
  const { socket, status, rest } = await rawUpgrade(port, '/ws', rawHeaderLines);
  if (status !== 101) { socket.destroy(); return { upgraded: false, status, close: null }; }
  socket.write(maskFrame(helloBadTicket()));
  const close = await readUntilClose(socket, rest);
  socket.destroy();
  return { upgraded: true, status, close };
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

console.log('B2: SEC-07 多值头/空值头真实服务行为（--auth-header X-Remote-User）');
const inst = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--credential', CRED, '--auth-header', 'X-Remote-User', '--', 'bash', '--norc', '--noprofile']);
try {
  // C1: 两行同名头（alice 在前 bob 在后）→ remote_user 取首值 alice，bob 不出现
  const mark1 = inst.stderrText().length;
  const p1 = await probeAuthFailed(inst.port, ['X-Remote-User: alice', 'X-Remote-User: bob']);
  await sleep(150);
  const tail1 = inst.stderrText().slice(mark1);
  check('B2a', '多值头 WS upgrade 101 + auth_failed close 到达',
    p1.upgraded && p1.close !== null && p1.close.code === 1008,
    `101=${p1.upgraded} code=${p1.close?.code ?? 'none'} reason=${p1.close?.reason ?? ''}`);
  check('B2b', '事件行 remote_user 取首值(=alice 形状)且无第二值泄漏',
    /remote_user=alice/.test(tail1) && !/bob/.test(tail1),
    `首值=${/remote_user=alice/.test(tail1)} 二值缺席=${!/bob/.test(tail1)}`);

  // C2: 空串头值 → sanitize 后空 → 事件行不出 remote_user 键（与缺席同态）
  const mark2 = inst.stderrText().length;
  const p2 = await probeAuthFailed(inst.port, ['X-Remote-User:']);
  await sleep(150);
  const tail2 = inst.stderrText().slice(mark2);
  check('B2c', '空值头 WS upgrade 101 + auth_failed close 到达',
    p2.upgraded && p2.close !== null && p2.close.code === 1008,
    `101=${p2.upgraded} code=${p2.close?.code ?? 'none'}`);
  check('B2d', '空值头事件行不出 remote_user 键(与缺席同态)',
    !/remote_user/.test(tail2) && /auth_failed/.test(tail2),
    `键缺席=${!/remote_user/.test(tail2)} 事件行存在=${/auth_failed/.test(tail2)}`);
} finally {
  inst.child.kill('SIGKILL');
}
const failed = results.filter((r) => !r).length;
console.log(`结果: ${results.length - failed}/${results.length} PASS`);
process.exit(failed ? 1 : 0);
