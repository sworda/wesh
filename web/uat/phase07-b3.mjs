// Phase 07 UAT B3 自动化证据脚本（Linux 侧运行）
// OPS-04: ① --cwd 符号链接按内核语义解析；② --term 任意字符串不校验（$TERM 原样）；
//         ③ --stop-timeout 极大值只推迟 KILL 不阻塞 exitf（子进程死亡即时退出）
import { spawn, execSync } from 'node:child_process';
import { symlinkSync, rmSync, existsSync } from 'node:fs';

const WESH = '/tmp/wesh-uat/wesh';
const enc = new TextEncoder();
const dec = new TextDecoder();
const OUTPUT = 0x30, HELLO = 0x48, WELCOME = 0x57;
const SUBPROTOCOL = 'wesh.v1';

const results = [];
const check = (id, name, ok, detail = '') => {
  results.push(ok);
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
const concat = (...parts) => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) { out.set(p, off); off += p.length; }
  return out;
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const helloFrame = () => concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols: 80, rows: 24 })));
const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');

function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, args, { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '', stdoutBuf = '';
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`启动超时: stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      stdoutBuf += d.toString();
      const m = /listening on https?:\/\/[^\s]+:(\d+)/.exec(stdoutBuf);
      if (m) { clearTimeout(to); setTimeout(() => resolve({ port: +m[1], child, stderrText: () => stderr }), 50); }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

function dialHello(port) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame());
    ws.onerror = () => reject(new Error('WS 连接失败'));
    const watchdog = setTimeout(() => reject(new Error('握手总超时')), 10000);
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) { clearInterval(poll); clearTimeout(watchdog); resolve({ ws, frames }); }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); clearTimeout(watchdog); reject(new Error(`握手被关闭 code=${ev.code}`)); };
  });
}

const sendInput = (ws, text) => ws.send(concat(new Uint8Array([0x30]), enc.encode(text)));
async function waitOutput(frames, fromIdx, marker, timeoutMs = 5000) {
  const t0 = Date.now();
  while (Date.now() - t0 < timeoutMs) {
    if (outputText(frames, fromIdx).includes(marker)) return true;
    await sleep(50);
  }
  return false;
}
function waitExit(child, timeoutMs) {
  return new Promise((resolve) => {
    const to = setTimeout(() => resolve(null), timeoutMs);
    child.once('exit', (code, signal) => { clearTimeout(to); resolve({ code, signal }); });
  });
}

console.log('B3: OPS-04 symlink cwd / 任意 TERM / 极大 stop-timeout');

// ① --cwd 符号链接：chdir 按内核语义解析，bash getcwd 得物理路径
const LINK = '/tmp/wesh-uat/b3-cwd-link';
rmSync(LINK, { force: true, recursive: true });
symlinkSync('/tmp', LINK);
{
  const inst = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--cwd', LINK, '--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(inst.port);
    const base = c.frames.length;
    sendInput(c.ws, 'echo B3C1:$(pwd -P)\r');
    const got = await waitOutput(c.frames, base, 'B3C1:/tmp');
    const leaked = outputText(c.frames, base).includes('B3C1:/tmp/wesh-uat/b3-cwd-link');
    check('B3a', '--cwd 符号链接启动且子进程 cwd 按内核语义解析(pwd -P=/tmp)',
      got && !leaked, `解析=${got} 链接路径泄漏=${leaked}`);
    c.ws.close();
  } finally { inst.child.kill('SIGKILL'); }
}
rmSync(LINK, { force: true, recursive: true });

// ② --term foobar：不校验，$TERM 原样
{
  const inst = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--term', 'foobar', '--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(inst.port);
    const base = c.frames.length;
    sendInput(c.ws, 'echo B3C2:$TERM\r');
    const got = await waitOutput(c.frames, base, 'B3C2:foobar');
    check('B3b', '--term 任意字符串不校验且 $TERM 原样为 foobar', got, `回读=${got}`);
    c.ws.close();
  } finally { inst.child.kill('SIGKILL'); }
}

// ③ --stop-timeout 1h：子进程忽略 HUP 存活 → SIGTERM 关停启动 → 手动 kill 子进程 →
//    wesh 即时退出（补 KILL 是 AfterFunc 异步兜底，不阻塞 exitf）
{
  const inst = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--stop-timeout', '1h', '--', 'bash', '--norc', '--noprofile', '-c', 'trap "" HUP; exec sleep 600']);
  const weshPid = inst.child.pid;
  await sleep(300); // 子进程(exec sleep 600)落定
  const childPid = execSync(`ps -o pid= --ppid ${weshPid}`).toString().trim().split(/\s+/)[0];
  const t0 = Date.now();
  inst.child.kill('SIGTERM');
  await sleep(400); // 关停序列启动：1001 广播 + HUP 发进程组（sleep 忽略）；1h KILL 已挂起
  execSync(`kill -9 ${childPid}`); // 中途手动 kill 子进程
  const exit = await waitExit(inst.child, 10000);
  const elapsed = Date.now() - t0;
  check('B3c', 'stop-timeout 1h 下子进程死亡后 wesh 即时退出(不等满 1h)且退出 255',
    exit !== null && exit.code === 255 && elapsed < 5000,
    `exit=${exit?.code ?? 'timeout'} elapsed=${elapsed}ms`);
  if (exit === null) inst.child.kill('SIGKILL');
}

const failed = results.filter((r) => !r).length;
console.log(`结果: ${results.length - failed}/${results.length} PASS`);
process.exit(failed ? 1 : 0);
