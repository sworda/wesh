// 最小复现探针（Linux 侧 loopback 运行）：per-client wesh + herdr，裸 WS attach
// 166x61 → 首帧落定 → +8s 后 INPUT "echo MARK:$fish_pid\r" → 观测 30s 内输出帧：
// MARK 输出是否到达、字节是否停流。若 loopback 复现 = 纯 herdr/服务端问题，
// 与浏览器/转发器/Windows 无关。零依赖 Node ≥22。
import { spawn } from 'node:child_process';
import os from 'node:os';

const WESH = '/tmp/wesh-uat/wesh';
const HERDR = `${os.homedir()}/.local/bin/herdr`;
const SESSION = `wesh-uat-minrepro-${process.pid}`;
const CRED = 'user:pass';
const SUBPROTOCOL = 'wesh.v1';
const OUTPUT = 0x30, HELLO = 0x48, WELCOME = 0x57;
const enc = new TextEncoder();

const concat = (...ps) => { const n = ps.reduce((a, p) => a + p.length, 0); const out = new Uint8Array(n); let o = 0; for (const p of ps) { out.set(p, o); o += p.length; } return out; };
const helloFrame = (cols, rows) => concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols, rows })));
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function startWesh() {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', '--session-mode', 'per-client', '--writable', '--insecure-http', '--', HERDR, '--session', SESSION], { stdio: ['ignore', 'pipe', 'pipe'] });
    let buf = '';
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error('wesh 启动超时')); }, 8000);
    child.stderr.on('data', () => {});
    child.stdout.on('data', (d) => {
      buf += d.toString();
      const m = /listening on (https?):\/\/[^\s]+:(\d+)/.exec(buf);
      if (m) { clearTimeout(to); resolve({ port: Number(m[2]), kill: () => child.kill('SIGKILL') }); }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

function dialHello(port, cols, rows) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    let bytes = 0, frameCount = 0, text = '';
    const frames = [];
    ws.onmessage = (ev) => { const f = new Uint8Array(ev.data); frames.push(f); bytes += f.length; frameCount++; if (f[0] === OUTPUT) text += new TextDecoder().decode(f.subarray(1)); };
    ws.onopen = () => ws.send(helloFrame(cols, rows));
    ws.onerror = () => reject(new Error('WS 连接失败'));
    const watchdog = setTimeout(() => reject(new Error('10s 未收到 Welcome')), 10000);
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) { clearInterval(poll); clearTimeout(watchdog); resolve({ ws, stats: { get bytes() { return bytes; }, get count() { return frameCount; }, get text() { return text; } } }); }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); clearTimeout(watchdog); reject(new Error(`握手关闭 code=${ev.code}`)); };
  });
}

const { port, kill } = await startWesh();
console.log(`wesh listening :${port}`);
const { ws, stats } = await dialHello(port, 166, 61);
let b1 = 0, b2 = 0;
do { b1 = stats.bytes; await sleep(150); b2 = stats.bytes; } while (b1 !== b2 || b1 === 0);
console.log(`首帧落定: frames=${stats.count} bytes=${stats.bytes}`);
await sleep(8000); // 与 pw 脚本同款 settle
console.log(`settle 后: frames=${stats.count} bytes=${stats.bytes}`);

const mark = `MR${Math.random().toString(36).slice(2, 6).toUpperCase()}`;
const t0 = Date.now();
ws.send(concat(new Uint8Array([OUTPUT]), enc.encode(`echo ${mark}:$fish_pid\r`)));
console.log(`INPUT 已发: echo ${mark}:$fish_pid\\r`);

let arrived = -1, lastBytes = stats.bytes, lastLog = Date.now(), plateauAt = -1;
while (Date.now() - t0 < 30000) {
  if (new RegExp(`${mark}:(\\d+)`).test(stats.text)) { arrived = Date.now() - t0; break; }
  if (Date.now() - lastLog > 2000) {
    lastLog = Date.now();
    const cur = stats.bytes;
    console.log(`  +${((Date.now() - t0) / 1000).toFixed(1)}s frames=${stats.count} bytes=${cur} Δ=${cur - lastBytes}`);
    if (cur === lastBytes && plateauAt < 0) plateauAt = Date.now() - t0;
    lastBytes = cur;
  }
  await sleep(100);
}
console.log(arrived >= 0
  ? `一发到达: +${arrived}ms`
  : `30s 未到达——流停摆复现（字节平台起点 ~+${plateauAt}ms, frames=${stats.count} bytes=${stats.bytes}）`);

if (arrived >= 0) {
  const mark2 = `MR2${Math.random().toString(36).slice(2, 5).toUpperCase()}`;
  const t2 = Date.now();
  ws.send(concat(new Uint8Array([OUTPUT]), enc.encode(`echo ${mark2}:$fish_pid\r`)));
  while (Date.now() - t2 < 10000) { if (new RegExp(`${mark2}:(\\d+)`).test(stats.text)) { console.log(`二发到达: +${Date.now() - t2}ms（流存活）`); break; } await sleep(100); }
  if (!new RegExp(`${mark2}:(\\d+)`).test(stats.text)) console.log('二发 10s 未到达');
} else {
  const mark3 = `MR3${Math.random().toString(36).slice(2, 5).toUpperCase()}`;
  const t3 = Date.now();
  ws.send(concat(new Uint8Array([OUTPUT]), enc.encode(`echo ${mark3}:$fish_pid\r`)));
  while (Date.now() - t3 < 15000) { if (new RegExp(`${mark3}:(\\d+)`).test(stats.text)) { console.log(`补发到达: +${Date.now() - t3}ms（流被后续输入激活）`); break; } await sleep(100); }
  if (!new RegExp(`${mark3}:(\\d+)`).test(stats.text)) console.log('补发 15s 仍未到达（流彻底死）');
}

ws.close();
kill();
console.log('MINREPRO_DONE');
