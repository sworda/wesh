// Phase 2 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket）。
// 覆盖 02-UAT.md 中可用无头 WS 客户端验证的协议断言；浏览器渲染层
// （[ro] 标题、onclose 文案面板、resize 视觉重绘）仍由人工确认。
//
// 运行：node web/uat/phase02.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
import { spawn } from 'node:child_process';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
const SUBPROTOCOL = 'wesh.v1';

const enc = new TextEncoder();
const dec = new TextDecoder();
const concat = (...parts) => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) { out.set(p, off); off += p.length; }
  return out;
};
const helloFrame = (version = SUBPROTOCOL) =>
  concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({ version, cols: 80, rows: 24 })));

const results = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok, detail });
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};

// 启动 wesh 实例，解析实际端口，返回 { port, kill }
function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, ['--port', '0', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${args.join(' ')}; stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      const m = /listening on http:\/\/[^\s]+:(\d+)/.exec(d.toString());
      if (m) {
        clearTimeout(to);
        resolve({ port: Number(m[1]), kill: () => child.kill('SIGKILL'), child });
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// 建立 WS 连接并完成 Hello 握手；返回 { ws, frames } ，frames 持续累积收到的二进制帧
function dialHello(port, { version = SUBPROTOCOL, sendHello = true } = {}) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => { if (sendHello) ws.send(helloFrame(version)); };
    ws.onerror = () => reject(new Error('WS 连接失败'));
    // Welcome 到达即视为握手完成
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) { clearInterval(poll); resolve({ ws, frames }); }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); reject(new Error(`握手被关闭 code=${ev.code} reason=${ev.reason}`)); };
  });
}

const waitClose = (ws, timeoutMs) => new Promise((resolve) => {
  const to = setTimeout(() => resolve(null), timeoutMs);
  ws.onclose = (ev) => { clearTimeout(to); resolve({ code: ev.code, reason: ev.reason }); };
});

const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');

// ---------- 场景 1：ro 默认（T1 协议层 + T3 协议层 + 保活反证） ----------
async function scenarioRO() {
  console.log('场景 1: wesh -- bash（ro 默认）');
  const inst = await startWesh(['--', 'bash', '--norc', '--noprofile']);
  try {
    const { ws, frames } = await dialHello(inst.port);

    // T1a: 首帧必须是 Welcome（服务端在 Hello 前不得发数据帧；RESIZE/OUTPUT 抢跑即 FAIL）
    check('T1a', '服务端首帧为 Welcome（无 OUTPUT/RESIZE 抢跑）', frames[0]?.[0] === WELCOME,
      `首帧类型 0x${frames[0]?.[0]?.toString(16)}`);

    // T1b: Welcome.mode === 'ro'
    const welcome = JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));
    check('T1b', "Welcome(mode=ro)", welcome.mode === 'ro', `mode=${welcome.mode}`);

    // T1c: ro 下 INPUT 被服务端丢弃（标记串不回显）
    await sleep(1200); // 排空 bash 启动输出基线
    const baseline = frames.length;
    const MARK = 'UAT_MARK_RO_7f3a9';
    ws.send(concat(new Uint8Array([INPUT]), enc.encode(`echo ${MARK}\n`)));
    await sleep(1500);
    const echo = outputText(frames, baseline);
    check('T1c', 'ro 模式 INPUT 被丢弃（无回显）', !echo.includes(MARK),
      echo.includes(MARK) ? '标记串被回显！' : '标记串未出现在输出');

    // T3a: ro 下 RESIZE 放行（连接不因 RESIZE 被关闭）
    ws.send(concat(new Uint8Array([RESIZE]), enc.encode(JSON.stringify({ cols: 100, rows: 40 }))));
    await sleep(500);
    check('T3a', 'ro 模式 RESIZE 帧放行（连接存活）', ws.readyState === WebSocket.OPEN);

    // T1d: 保活反证——5s 间隔 ping，pongTimeout 10s；存活 >11s 即两轮 ping/pong 成功
    await sleep(9500);
    check('T1d', 'ping/pong 保活（11s+ 连接未被 CloseNow）', ws.readyState === WebSocket.OPEN);
    ws.close();
  } finally {
    inst.kill();
  }
}

// ---------- 场景 2：--writable（T2 协议层） ----------
async function scenarioRW() {
  console.log('场景 2: wesh --writable -- bash');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const { ws, frames } = await dialHello(inst.port);
    const welcome = JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));
    check('T2a', "Welcome(mode=rw)", welcome.mode === 'rw', `mode=${welcome.mode}`);

    await sleep(1200);
    const baseline = frames.length;
    const MARK = 'UAT_MARK_RW_2b8e1';
    ws.send(concat(new Uint8Array([INPUT]), enc.encode(`echo ${MARK}\n`)));
    await sleep(1500);
    const echo = outputText(frames, baseline);
    check('T2b', 'rw 模式输入正常回显（标记串出现在输出）', echo.includes(MARK));
    ws.close();
  } finally {
    inst.kill();
  }
}

// ---------- 场景 4a：子进程退出 → close 1000 ----------
async function scenarioExit() {
  console.log('场景 4a: wesh -- sleep 2（子进程退出）');
  const inst = await startWesh(['--', 'sleep', '2']);
  try {
    const { ws } = await dialHello(inst.port);
    const close = await waitClose(ws, 6000);
    check('T4a', '子进程退出 → close 1000（Session ended）', close?.code === 1000,
      `code=${close?.code} reason=${close?.reason ?? ''}`);
  } finally {
    inst.kill();
  }
}

// ---------- 场景 4b：伪造 wesh.v9 → Error(version_mismatch) + 1008 ----------
async function scenarioVersionMismatch() {
  console.log('场景 4b: Hello version=wesh.v9');
  const inst = await startWesh(['--', 'sleep', '30']);
  try {
    const ws = new WebSocket(`ws://127.0.0.1:${inst.port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame('wesh.v9'));
    const close = await waitClose(ws, 5000);
    const err = frames.find((f) => f[0] === ERROR);
    const errObj = err ? JSON.parse(dec.decode(err.subarray(1))) : null;
    check('T4b', '版本不符 → Error(version_mismatch) + close 1008',
      errObj?.code === 'version_mismatch' && close?.code === 1008,
      `error=${JSON.stringify(errObj)} close=${close?.code}`);
  } finally {
    inst.kill();
  }
}

// ---------- 场景 4c：不发 Hello → 5s 超时 1008 ----------
async function scenarioHelloTimeout() {
  console.log('场景 4c: 连接后不发 Hello（hello_timeout）');
  const inst = await startWesh(['--', 'sleep', '30']);
  try {
    const ws = new WebSocket(`ws://127.0.0.1:${inst.port}/ws`, [SUBPROTOCOL]);
    await new Promise((r) => { ws.onopen = r; });
    const t0 = Date.now();
    const close = await waitClose(ws, 8000);
    const elapsed = Date.now() - t0;
    check('T4c', '无 Hello → ~5s 被 1008 关闭',
      close?.code === 1008 && elapsed >= 4500 && elapsed <= 7500,
      `code=${close?.code} reason=${close?.reason ?? ''} 耗时=${elapsed}ms`);
  } finally {
    inst.kill();
  }
}

// ---------- 场景 5：单客户端（第二连接 409） ----------
async function scenarioSingleClient() {
  console.log('场景 5: 第二连接 409');
  const inst = await startWesh(['--', 'sleep', '30']);
  try {
    const { ws } = await dialHello(inst.port); // 主连接占用
    const { default: http } = await import('node:http');
    const status = await new Promise((resolve, reject) => {
      const key = Buffer.from('uat-second-client-key').toString('base64');
      const req = http.request({
        host: '127.0.0.1', port: inst.port, path: '/ws',
        headers: {
          Connection: 'Upgrade', Upgrade: 'websocket',
          'Sec-WebSocket-Key': key, 'Sec-WebSocket-Version': '13',
          'Sec-WebSocket-Protocol': SUBPROTOCOL,
        },
      });
      req.on('response', (res) => { res.resume(); resolve(res.statusCode); });
      req.on('upgrade', () => resolve(101));
      req.on('error', reject);
      req.end();
    });
    check('T5', '第二客户端握手 → HTTP 409（Unable to connect）', status === 409, `status=${status}`);
    ws.close();
  } finally {
    inst.kill();
  }
}

const scenarios = [scenarioRO, scenarioRW, scenarioExit, scenarioVersionMismatch, scenarioHelloTimeout, scenarioSingleClient];
let failed = 0;
for (const s of scenarios) {
  try {
    await s();
  } catch (e) {
    failed++;
    console.log(`  FAIL  场景异常: ${e.message}`);
  }
  await sleep(300);
}
const passed = results.filter((r) => r.ok).length;
console.log(`\n结果: ${passed}/${results.length} 协议断言通过${failed ? `，${failed} 个场景异常` : ''}`);
process.exit(passed === results.length && failed === 0 ? 0 : 1);
