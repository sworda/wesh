// 最小复现（Windows 侧运行）：过本机转发器连 LAN wesh，裸 WS（无浏览器）attach
// 166x61 → 首帧落定 → +8s INPUT echo → 观测输出流。
// A/B 开关 MINREPRO_TICKET=1：wesh 带 --credential + ticket 认证（与浏览器同路径）；
// 默认 --no-auth 直连。判决 ticket-attach 路径是否为输出流停摆的必要条件。
import { sleep } from './lib/check.mjs';
import { Forwarder } from './lib/forwarder.mjs';
import { TARGET_HOST, TARGET_PORT, ssh, ensureRunSh, startWesh, stopWesh } from './lib/server.mjs';

const PORT_BASE = parseInt(process.env.WESH_UAT_PORT_BASE || '17681', 10);
const HERDR = process.env.WESH_UAT_HERDR || '~/.local/bin/herdr';
const SESSION = `wesh-uat-p14-minwin-${Date.now()}`;
const CRED = 'user:pass';
const SUBPROTOCOL = 'wesh.v1';
const OUTPUT = 0x30, HELLO = 0x48, WELCOME = 0x57;
const enc = new TextEncoder();
const dec = new TextDecoder();
const USE_TICKET = process.env.MINREPRO_TICKET === '1';

const concat = (...ps) => { const n = ps.reduce((a, p) => a + p.length, 0); const out = new Uint8Array(n); let o = 0; for (const p of ps) { out.set(p, o); o += p.length; } return out; };
const helloFrame = (cols, rows, ticket) => concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify(ticket === undefined ? { version: SUBPROTOCOL, cols, rows } : { version: SUBPROTOCOL, cols, rows, ticket })));

function dialHello(port, cols, rows, ticket) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    let bytes = 0, frameCount = 0, text = '', welcome = null;
    const frames = [];
    ws.onmessage = (ev) => { const f = new Uint8Array(ev.data); frames.push(f); bytes += f.length; frameCount++; if (f[0] === WELCOME) { try { welcome = JSON.parse(dec.decode(f.subarray(1))); } catch { welcome = null; } } if (f[0] === OUTPUT) text += dec.decode(f.subarray(1)); };
    ws.onopen = () => ws.send(helloFrame(cols, rows, ticket));
    ws.onerror = () => reject(new Error('WS 连接失败'));
    const watchdog = setTimeout(() => reject(new Error('10s 未收到 Welcome')), 10000);
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) { clearInterval(poll); clearTimeout(watchdog); resolve({ ws, welcome, stats: { get bytes() { return bytes; }, get count() { return frameCount; }, get text() { return text; } } }); }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); clearTimeout(watchdog); reject(new Error(`握手关闭 code=${ev.code}`)); };
  });
}

const fwd = new Forwarder(PORT_BASE, TARGET_HOST, TARGET_PORT);
try {
  await ensureRunSh();
  const credArgs = USE_TICKET ? '--credential user:pass' : '--no-auth';
  await startWesh(
    `--session-mode per-client --writable --insecure-http ${credArgs} -- ${HERDR} --session ${SESSION}`,
  );
  await fwd.start();
  let ticket;
  if (USE_TICKET) {
    const resp = await fetch(`http://127.0.0.1:${PORT_BASE}/api/attach`, { method: 'POST', headers: { Authorization: 'Basic ' + Buffer.from(CRED).toString('base64') } });
    if (!resp.ok) throw new Error(`attach HTTP ${resp.status}`);
    ticket = (await resp.json()).ticket;
    console.log('ticket 取得（值不入日志）');
  }
  const { ws, welcome, stats } = await dialHello(PORT_BASE, 166, 61, ticket);
  console.log(`Welcome: mode=${welcome?.mode} session=${welcome?.session}`);
  let b1 = 0, b2 = 0;
  do { b1 = stats.bytes; await sleep(150); b2 = stats.bytes; } while (b1 !== b2 || b1 === 0);
  console.log(`首帧落定: frames=${stats.count} bytes=${stats.bytes}`);
  await sleep(8000);
  console.log(`settle 后: frames=${stats.count} bytes=${stats.bytes}`);

  const mark = `MR${Math.random().toString(36).slice(2, 6).toUpperCase()}`;
  const t0 = Date.now();
  ws.send(concat(new Uint8Array([OUTPUT]), enc.encode(`echo ${mark}:$fish_pid\r`)));
  console.log(`INPUT 已发: echo ${mark}:$fish_pid\\r`);

  let arrived = -1, lastBytes = stats.bytes, lastLog = Date.now();
  while (Date.now() - t0 < 30000) {
    if (new RegExp(`${mark}:(\\d+)`).test(stats.text)) { arrived = Date.now() - t0; break; }
    if (Date.now() - lastLog > 2000) {
      lastLog = Date.now();
      const cur = stats.bytes;
      console.log(`  +${((Date.now() - t0) / 1000).toFixed(1)}s frames=${stats.count} bytes=${cur} Δ=${cur - lastBytes}`);
      lastBytes = cur;
    }
    await sleep(100);
  }
  console.log(`[${USE_TICKET ? 'TICKET' : 'NOAUTH'}] 一发到达: +${arrived}ms${arrived < 0 ? '（30s 停摆）' : ''}`);

  if (arrived >= 0) {
    const mark2 = `MR2${Math.random().toString(36).slice(2, 5).toUpperCase()}`;
    const t2 = Date.now();
    ws.send(concat(new Uint8Array([OUTPUT]), enc.encode(`echo ${mark2}:$fish_pid\r`)));
    while (Date.now() - t2 < 10000) { if (new RegExp(`${mark2}:(\\d+)`).test(stats.text)) { console.log(`二发到达: +${Date.now() - t2}ms（流存活）`); break; } await sleep(100); }
    if (!new RegExp(`${mark2}:(\\d+)`).test(stats.text)) console.log('二发 10s 未到达');
  }

  ws.close();
} catch (e) {
  console.log(`异常: ${e.message}`);
} finally {
  await fwd.stop().catch(() => {});
  await stopWesh().catch(() => {});
  await ssh(`bash -lc '${HERDR} session stop ${SESSION} || true; ${HERDR} session delete ${SESSION} || true'`).catch(() => {});
}
console.log('MINWIN_DONE');
