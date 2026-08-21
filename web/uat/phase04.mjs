// Phase 4 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch）。
// 覆盖 FE-07 可自动化协议面：Welcome prefs 形状（omitempty 缺席回归 / 逐键注入 /
// theme 对象透传 / osc52 下发 / last-wins）与 --client-option 启动校验拒绝
// （D-12 安全不对称 / D-15 白名单 + JSON fail-fast）。渲染层（URL query 覆盖、
// prefs 应用后视觉效果、IME/剪贴板/浮层）由 04-UAT.md 人工确认。
//
// 红线（phase03.mjs 延伸到测试输出）：prefs/theme 值内容只作断言材料，永不进入
// check detail 或任何控制台输出——detail 只打印状态码/布尔/键形状（theme 等用户
// 配置虽非凭据，仍只打形状保持纪律一致；测试输出同样可能进 CI 日志）。
//
// 场景隔离纪律（phase03.mjs S1f 先例沿用）：每个需 WS 的场景独立 spawn 实例——
// 多客户端下同进程多 WS 建连已是 Phase 5 特性，独立 spawn 仅为场景间零状态干扰。
//
// 运行：node web/uat/phase04.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
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
// Hello 载荷 {version,cols,rows}——loopback 无认证直连，无 ticket 键
// （omitempty 对称形态：无认证模式前端不出 ticket 键）。
const helloFrame = (version = SUBPROTOCOL) =>
  concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({ version, cols: 80, rows: 24 })));

const results = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok, detail });
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};

// 启动 wesh 实例，解析实际端口，返回 { port, scheme, kill }。
// 所有场景显式 --bind 127.0.0.1 + --port 0（loopback 随机端口，与用户服务零干扰）。
function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${args.join(' ')}; stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      // scheme 感知启动行解析（照 phase03.mjs 形态）
      const m = /listening on (https?):\/\/[^\s]+:(\d+)/.exec(d.toString());
      if (m) {
        clearTimeout(to);
        resolve({ port: Number(m[2]), scheme: m[1], kill: () => child.kill('SIGKILL'), child });
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

// 启动校验拒绝路径 helper：进程预期 3s 内自行退出（拒绝路径不打印 listening 行
// 而是直接非零退出，startWesh 等端口必然超时，必须走此 spawn-exit 形态）。
function spawnExpectExit(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, args, { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    child.stderr.on('data', (d) => { stderr += d; });
    const to = setTimeout(() => {
      child.kill('SIGKILL');
      reject(new Error(`wesh 未在 3s 内退出（拒绝路径应早退）: ${args.join(' ')}`));
    }, 3000);
    child.on('exit', (code) => { clearTimeout(to); resolve({ code, stderr }); });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// 建立 WS 连接并完成 Hello 握手（无 ticket——loopback 无认证直连）；
// 返回 { ws, frames }，frames 持续累积
function dialHello(port) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame());
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

// 取 frames 中首帧 WELCOME 的 JSON 载荷
const welcomeOf = (frames) => JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));

// detail 键形状：键名可打、值不打（红线）
const keyShape = (prefs) => Object.keys(prefs ?? {}).sort().join(',');

// ---------- S1：默认无 prefs（omitempty 缺席回归，D-13 旧前端零漂移） ----------
async function s1DefaultNoPrefs() {
  console.log('S1: 默认形态（无任何偏好 flag）→ Welcome 无 prefs 键');
  const inst = await startWesh(['--', 'bash', '--norc', '--noprofile']);
  try {
    const { ws, frames } = await dialHello(inst.port);
    const welcome = welcomeOf(frames);
    const absent = !('prefs' in welcome);
    check('S1', '无任何偏好 flag → Welcome JSON 无 prefs 键（omitempty 缺席回归）',
      absent,
      `mode=${welcome.mode} prefs键缺席=${absent}`);
    ws.close();
    await waitClose(ws, 3000); // 关闭握手落定后再 SIGKILL（不断言，仅收口）
  } finally {
    inst.kill();
  }
}

// ---------- S2：--client-option 两键注入 ----------
async function s2ClientOptionInject() {
  console.log('S2: --client-option fontSize/cursorBlink 两键注入');
  const inst = await startWesh(['--client-option', 'fontSize=18', '--client-option', 'cursorBlink=false', '--', 'bash', '--norc', '--noprofile']);
  try {
    const { ws, frames } = await dialHello(inst.port);
    const welcome = welcomeOf(frames);
    const fontSizeOk = welcome.prefs?.fontSize === 18;
    const blinkOk = welcome.prefs?.cursorBlink === false;
    check('S2', '--client-option 两键注入 → prefs 逐键值相等',
      fontSizeOk && blinkOk,
      `keys=${keyShape(welcome.prefs)} fontSize等式=${fontSizeOk} cursorBlink等式=${blinkOk}`);
    ws.close();
    await waitClose(ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S3：theme 完整 JSON 对象原样透传（D-19） ----------
async function s3ThemeObject() {
  console.log('S3: --client-option theme JSON 对象');
  const inst = await startWesh(['--client-option', 'theme={"background":"#000000"}', '--', 'bash', '--norc', '--noprofile']);
  try {
    const { ws, frames } = await dialHello(inst.port);
    const welcome = welcomeOf(frames);
    const theme = welcome.prefs?.theme;
    const isObj = typeof theme === 'object' && theme !== null && !Array.isArray(theme);
    const bgOk = isObj && theme.background === '#000000';
    check('S3', 'theme 完整 JSON 对象 → prefs.theme 为对象且 background 键值相等',
      isObj && bgOk,
      `keys=${keyShape(welcome.prefs)} theme为对象=${isObj} background等式=${bgOk}`);
    ws.close();
    await waitClose(ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S4：--osc52 注入（D-12 服务端专有开关经 prefs 下发） ----------
// Phase 5 适配（D-13 旁观端 osc52 强制关）：prefs 按 mode 分化双档——ro 端不再下发
// osc52，下发通道断言改在 rw 端（--writable）进行；断言面零丢失（键存在性与值等式不变）。
async function s4Osc52() {
  console.log('S4: --osc52 → prefs.osc52 === true（rw 端，D-13 后 osc52 不下发 ro 档）');
  const inst = await startWesh(['--osc52', '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const { ws, frames } = await dialHello(inst.port);
    const welcome = welcomeOf(frames);
    const oscOk = welcome.prefs?.osc52 === true;
    check('S4', '--osc52 → prefs.osc52 === true（D-12 服务端专有开关下发）',
      oscOk,
      `keys=${keyShape(welcome.prefs)} osc52布尔=${oscOk}`);
    ws.close();
    await waitClose(ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S5：--client-option 与 --osc52 组合（两键俱在） ----------
// Phase 5 适配同 S4（D-13）：osc52 仅 rw 档下发，故 spawn 加 --writable。
async function s5Combo() {
  console.log('S5: --client-option + --osc52 组合（rw 端）');
  const inst = await startWesh(['--client-option', 'fontSize=20', '--osc52', '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const { ws, frames } = await dialHello(inst.port);
    const welcome = welcomeOf(frames);
    const fontSizeOk = welcome.prefs?.fontSize === 20;
    const oscOk = welcome.prefs?.osc52 === true;
    check('S5', '组合注入 → fontSize 与 osc52 两键俱在且值正确',
      fontSizeOk && oscOk,
      `keys=${keyShape(welcome.prefs)} fontSize等式=${fontSizeOk} osc52布尔=${oscOk}`);
    ws.close();
    await waitClose(ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S6：重复 key last-wins ----------
async function s6LastWins() {
  console.log('S6: 重复 --client-option key → last-wins');
  const inst = await startWesh(['--client-option', 'fontSize=14', '--client-option', 'fontSize=22', '--', 'bash', '--norc', '--noprofile']);
  try {
    const { ws, frames } = await dialHello(inst.port);
    const welcome = welcomeOf(frames);
    const lastWins = welcome.prefs?.fontSize === 22;
    check('S6', 'fontSize 重复两给 → prefs.fontSize 取后者（last-wins）',
      lastWins,
      `keys=${keyShape(welcome.prefs)} lastWins等式=${lastWins}`);
    ws.close();
    await waitClose(ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- E1-E4：--client-option 启动校验拒绝（D-12/D-15，spawn-exit 形态） ----------
// parse 期 fail-fast：拒绝发生在 listen 之前，exit 2 + stderr 错误类别文案
// （文案只含 key 名与错误类别，不含值内容——04-01 记录式上报红线）。
async function scenarioClientOptionRejects() {
  console.log('E1-E4: --client-option 启动校验拒绝矩阵');
  // E1: 白名单外 key（allowProposedApi 危险面，D-14）→ exit 2 + invalid --client-option key
  const r1 = await spawnExpectExit(['--client-option', 'allowProposedApi=true', '--', 'bash']);
  check('E1', '白名单外 key（allowProposedApi 危险面）→ exit 2 拒绝启动',
    r1.code === 2 && r1.stderr.includes('invalid --client-option key'),
    `exit=${r1.code} 文案=${r1.stderr.includes('invalid --client-option key')}`);

  // E2: 值非法 JSON → exit 2 + not valid JSON
  const r2 = await spawnExpectExit(['--client-option', 'fontSize=abc', '--', 'bash']);
  check('E2', '值非法 JSON → exit 2 拒绝启动',
    r2.code === 2 && r2.stderr.includes('not valid JSON'),
    `exit=${r2.code} 文案=${r2.stderr.includes('not valid JSON')}`);

  // E3: osc52 不在白名单（D-12 安全不对称——安全敏感项结构性排除出用户侧通道）
  const r3 = await spawnExpectExit(['--client-option', 'osc52=true', '--', 'bash']);
  check('E3', 'osc52 key 拒绝（D-12 安全不对称）→ exit 2 拒绝启动',
    r3.code === 2 && r3.stderr.includes('invalid --client-option key'),
    `exit=${r3.code} 文案=${r3.stderr.includes('invalid --client-option key')}`);

  // E4: 缺 '=' → exit 2 + must be key=value
  const r4 = await spawnExpectExit(['--client-option', 'fontSize', '--', 'bash']);
  check('E4', '缺 =（非 key=value 形态）→ exit 2 拒绝启动',
    r4.code === 2 && r4.stderr.includes('must be key=value'),
    `exit=${r4.code} 文案=${r4.stderr.includes('must be key=value')}`);
}

const scenarios = [s1DefaultNoPrefs, s2ClientOptionInject, s3ThemeObject, s4Osc52, s5Combo, s6LastWins, scenarioClientOptionRejects];
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
