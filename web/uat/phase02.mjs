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
    // D-03 适配（Phase 3 裸跑收口）：显式 loopback bind——无凭据起服的合法前提
    //（默认 0.0.0.0 无凭据自 Phase 3 起拒绝启动）；协议断言语义零改动。
    const child = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    let stdoutBuf = '';
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${args.join(' ')}; stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      // IN-04 累积缓冲后匹配（phase05.mjs 形态回填）：listening 行跨 chunk
      // 分块时逐 chunk 直接正则永不命中，8s 超时假失败
      stdoutBuf += d.toString();
      const m = /listening on http:\/\/[^\s]+:(\d+)/.exec(stdoutBuf);
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
    // IN-04 总超时 watchdog：被测二进制挂死（Welcome 永不到达）时 10s 拒绝而非永久悬挂
    const watchdog = setTimeout(() => {
      clearInterval(poll);
      reject(new Error('握手总超时：10s 未收到 Welcome'));
    }, 10000);
    // Welcome 到达即视为握手完成
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) { clearInterval(poll); clearTimeout(watchdog); resolve({ ws, frames }); }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); clearTimeout(watchdog); reject(new Error(`握手被关闭 code=${ev.code} reason=${ev.reason}`)); };
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

// ---------- 场景 5：多客户端（第二连接成功 + 断开后服务端存活可再 attach） ----------
// Phase 5 适配：409 单客户端门已拆除（Attach 守卫区③位换 max-clients 503 闸——
// 容量上限断言由 phase05.mjs S5 双点位承接，未丢失）；第二连接成功即多客户端特性
// 本身。生命周期预期改写：客户端断开不再使服务端退出（单次语义终结，服务端生命
// 周期只随子进程）——断言面由『拒绝/退出』改写为『存活、可再 attach』，数量守恒。
async function scenarioMultiClient() {
  console.log('场景 5: 多客户端（第二连接成功 + 断开后服务端存活可再 attach）');
  const inst = await startWesh(['--', 'sleep', '30']);
  try {
    const { ws: ws1 } = await dialHello(inst.port); // 主连接占用
    const { ws: ws2, frames: frames2 } = await dialHello(inst.port);
    const welcome2 = JSON.parse(dec.decode(frames2.find((f) => f[0] === WELCOME).subarray(1)));
    check('T5a', '第二客户端握手成功 → Welcome(mode=ro)（多客户端同会话，409 门 Phase 5 拆除）',
      ws1.readyState === WebSocket.OPEN && welcome2.mode === 'ro', `mode=${welcome2.mode}`);
    ws1.close(); ws2.close();
    await waitClose(ws1, 3000); await waitClose(ws2, 3000);
    // 全部客户端断开后服务端存活——可再 attach（D-10 唯一终结路径 = 子进程退出）
    const { ws: ws3, frames: frames3 } = await dialHello(inst.port);
    const welcome3 = JSON.parse(dec.decode(frames3.find((f) => f[0] === WELCOME).subarray(1)));
    check('T5b', '全部客户端断开后服务端存活、可再 attach（单次语义终结）', welcome3.mode === 'ro', `mode=${welcome3.mode}`);
    ws3.close();
    await waitClose(ws3, 3000);
  } finally {
    inst.kill();
  }
}

const scenarios = [scenarioRO, scenarioRW, scenarioExit, scenarioVersionMismatch, scenarioHelloTimeout, scenarioMultiClient];
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
