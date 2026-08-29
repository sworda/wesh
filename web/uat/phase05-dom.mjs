// Phase 5 渲染/交互逻辑面自动化 UAT（jsdom + Node 原生 WebSocket/fetch）。
//
// 定位：覆盖 05-UAT.md 人工清单 Test 3/4/5/6/7 的**代码逻辑面**——ro 三要素门控、
// 递补升格 UX、1013/503/无效链接三专版面板渲染与手动刷新链路，全部在无浏览器
// headless 环境断言（根 CODEBUDDY.md 四层测试策略第 3 层）。驱动方式与断言基建
// 照 phase04-dom.mjs：jsdom 加载真实构建产物 web/dist/index.html，注入 Node 原生
// WebSocket/fetch 与固定布局桩，连接真实 spawn 的 wesh 实例端到端断言。
//
// 本文件额外夹具：
//   - SpyWebSocket：记录全部上行帧（INPUT/RESIZE/HELLO），断言 ro 零上行；
//   - Atomics.wait 事件循环阻塞：undici 停止 drain TCP → 内核接收缓冲填满 →
//     服务端 outbox 涨满 → 真实 1013 踢出（phase05.mjs S6 rawStallClient 的
//     jsdom 侧等价物——jsdom 客户端无法 pause socket，阻塞事件循环同效）；
//   - opts.path：jsdom 页面 URL 携带 /s/{token}/ 路径段（分享链接进入链路）。
//
// D6（G-05-1 视口约束渲染，05-11 行为面回归锁）：窄 owner 钉会话尺寸 40x10 +
// 宽端 jsdom 旁观页（fit 80x24）——约束 rows / 会话 cols 折行（叠写等价物）/
// 升格解除约束三断言，与 phase05.mjs S10（协议面）、phase05-dims.mjs（终端核心层
// 等价锁 + 负对照）构成 G-05-1 三层自动化锁定。
//
// 不可测余项（任何 headless 方案结构性不可测，不在此声明覆盖）：浏览器原生 Basic
// 弹窗形态、多端像素级一致性、真实节流工具下的慢网——见 05-UAT.md 豁免注记。
//
// 运行：node web/uat/phase05-dom.mjs [wesh 二进制路径]（默认 /tmp/wesh-uat/wesh）
import { spawn } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { JSDOM } from 'jsdom';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';
const DIST = new URL('../dist/index.html', import.meta.url).pathname;

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57;
const SUBPROTOCOL = 'wesh.v1';
// UAT 专用凭据（phase05.mjs 同款；值不入任何断言输出——红线）
const UAT_CREDENTIAL = 'uat:uat-pass-x9';
const basicAuthHeader = () => 'Basic ' + Buffer.from(UAT_CREDENTIAL).toString('base64');

const enc = new TextEncoder();
const dec = new TextDecoder();

const results = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// parseEvents：stderr 混合流按行解析 JSON 事件（08-01 D-13 迁移后事件为 slog JSON
// 单行）——滤非 '{' 起始行；'{' 起始行非法 JSON 即抛错（带行号与行首 120 字符截断）。
// 08-05 补挂：D5 踢出检测从子串 'slow_consumer' 迁移到 detach reason=kick 字段断言
//（08-02 D-21 折入后 kick 不再单独打行，原文本行形态已终结——禁止子串断言 JSON 行）。
const parseEvents = (text) =>
  text.split('\n').flatMap((line, i) => {
    if (!line.startsWith('{')) return [];
    try {
      return [JSON.parse(line)];
    } catch (e) {
      throw new Error(`事件行非合法 JSON（第 ${i + 1} 行）: ${line.slice(0, 120)}: ${e.message}`);
    }
  });
async function waitFor(fn, label, timeout = 5000) {
  const t0 = Date.now();
  for (;;) {
    try { const v = fn(); if (v) return v; } catch { /* 断言中途态忽略 */ }
    if (Date.now() - t0 > timeout) throw new Error(`waitFor 超时: ${label}`);
    await sleep(25);
  }
}

// 启动 wesh 实例，解析端口与分享链接两行（phase05.mjs startWesh 同形态）。
// token 值只存闭包变量作断言材料——红线：永不进 check detail/控制台输出。
function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    let stdoutBuf = '';
    let settling = false;
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${args.join(' ')}; stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      stdoutBuf += d.toString();
      if (settling) return;
      const m = /listening on (https?):\/\/[^\s]+:(\d+)/.exec(stdoutBuf);
      if (m && stdoutBuf.includes('share read-only:')) {
        settling = true;
        clearTimeout(to);
        setTimeout(() => {
          const shareRO = /share read-only:\s+(\S+)/.exec(stdoutBuf)?.[1] ?? null;
          const shareRW = /share read-write:\s+(\S+)/.exec(stdoutBuf)?.[1] ?? null;
          resolve({ port: Number(m[2]), scheme: m[1], shareRO, shareRW, stderrText: () => stderr, kill: () => child.kill('SIGKILL'), child });
        }, 50);
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

const tokenFromUrl = (url) => /\/s\/([^/]+)\//.exec(url)[1];
const pathFromUrl = (url) => new URL(url).pathname;

// 原生 WS 客户端握手（owner 占位/洪水驱动等配角通道；可携 ticket）
function dialHello(port, { ticket, cols = 80, rows = 24 } = {}) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(concat(
      new Uint8Array([HELLO]),
      enc.encode(JSON.stringify(ticket === undefined ? { version: SUBPROTOCOL, cols, rows } : { version: SUBPROTOCOL, cols, rows, ticket }))));
    ws.onerror = () => reject(new Error('WS 连接失败'));
    const watchdog = setTimeout(() => reject(new Error('握手总超时')), 10000);
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) { clearInterval(poll); clearTimeout(watchdog); resolve({ ws, frames }); }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); clearTimeout(watchdog); reject(new Error(`握手被关闭 code=${ev.code}`)); };
  });
}
const concat = (...parts) => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) { out.set(p, off); off += p.length; }
  return out;
};
async function attachTicket(port, token) {
  const resp = await fetch(`http://127.0.0.1:${port}/api/attach`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ token }),
  });
  if (!resp.ok) throw new Error(`/api/attach 非 200: ${resp.status}`);
  return (await resp.json()).ticket;
}

// 加载 dist 到 jsdom 并执行 bundle。opts.path: 页面路径（/s/{token}/ 分享进入）；
// opts.auth: true 时 fetch 注入 Basic 头（模拟浏览器凭据缓存——原生 Basic 弹窗
// 本身是平台行为，jsdom 无导航不复现，注入头即其通过后的等价形态）；
// opts.clipboard 恒 'absent'（Phase 5 场景不断言剪贴板）。
async function loadTerminal(srv, opts = {}) {
  const html = readFileSync(DIST, 'utf8');
  const js = /<script type="module" crossorigin>([\s\S]*?)<\/script>/.exec(html)[1];
  const origin = `${srv.scheme}://${srv.host ?? '127.0.0.1'}:${srv.port}`;
  const url = `${origin}${opts.path ?? '/'}`;
  const dom = new JSDOM('', { url, pretendToBeVisual: true, runScripts: 'outside-only' });
  const { window } = dom;

  // ── 平台能力注入/桩 ──
  // SpyWebSocket：记录全部上行帧首字节（INPUT/RESIZE/HELLO 分派断言材料）
  const sentFrames = [];
  window.WebSocket = class extends WebSocket {
    constructor(...a) { super(...a); this.binaryType = 'arraybuffer'; }
    send(data) {
      const u8 = data instanceof ArrayBuffer ? new Uint8Array(data) : new Uint8Array(data.buffer ?? data);
      sentFrames.push(u8[0]);
      return super.send(data);
    }
  };
  const authFetch = (u, o = {}) => fetch(new URL(u, origin), {
    ...o, headers: { ...(o.headers ?? {}), Authorization: basicAuthHeader() },
  });
  window.fetch = opts.auth ? authFetch : ((u, o) => fetch(new URL(u, origin), o));
  window.TextEncoder = TextEncoder;
  window.TextDecoder = TextDecoder;
  window.matchMedia = (q) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, onchange: null, dispatchEvent: () => false });
  // 布局桩：720x408 视口、9x17 字符 → 恰 80x24（phase04-dom.mjs 约定）
  const RECT = { x: 0, y: 0, top: 0, left: 0, right: 720, bottom: 408, width: 720, height: 408, toJSON: () => ({}) };
  window.HTMLElement.prototype.getBoundingClientRect = function () { return RECT; };
  Object.defineProperty(window.HTMLElement.prototype, 'offsetWidth', { get() { return 720; }, configurable: true });
  Object.defineProperty(window.HTMLElement.prototype, 'offsetHeight', { get() { return 408; }, configurable: true });
  Object.defineProperty(window.HTMLElement.prototype, 'clientWidth', { get() { return 720; }, configurable: true });
  Object.defineProperty(window.HTMLElement.prototype, 'clientHeight', { get() { return 408; }, configurable: true });
  const metrics = (s) => ({ width: s.length * 9, fontBoundingBoxAscent: 14, fontBoundingBoxDescent: 3, actualBoundingBoxAscent: 14, actualBoundingBoxDescent: 3 });
  const ctx2d = { measureText: metrics, fillRect() {}, clearRect() {}, getImageData: (x, y, w, h) => ({ data: new Uint8ClampedArray(w * h * 4) }), putImageData() {}, createImageData: () => ({ data: new Uint8ClampedArray(0) }), setTransform() {}, drawImage() {}, save() {}, restore() {}, beginPath() {}, closePath() {}, rect() {}, clip() {}, fill() {}, translate() {}, scale() {}, rotate() {}, arc() {}, fillText() {}, strokeText() {} };
  window.HTMLCanvasElement.prototype.getContext = function (kind) { return kind === '2d' ? ctx2d : null; };
  window.OffscreenCanvas = class {
    constructor(w, h) { this.width = w; this.height = h; }
    getContext(kind) { return kind === '2d' ? ctx2d : null; }
  };
  const dims = { w: 720, h: 408 };
  const origGCS = window.getComputedStyle.bind(window);
  window.getComputedStyle = (el, pseudo) => {
    const cs = origGCS(el, pseudo);
    const origGPV = cs.getPropertyValue.bind(cs);
    cs.getPropertyValue = (prop) => {
      const v = origGPV(prop);
      if (v === '' && prop.startsWith('padding')) return '0px';
      if (v === '' && el.id === 'terminal') {
        if (prop === 'width') return `${dims.w}px`;
        if (prop === 'height') return `${dims.h}px`;
      }
      return v;
    };
    return cs;
  };

  // 观测钩子：console.info（ro 一次性提示断言通道）与异常
  const infos = [];
  const origInfo = window.console.info.bind(window.console);
  window.console.info = (...a) => { infos.push(a.map(String).join(' ')); };
  const warns = [];
  const origWarn = window.console.warn.bind(window.console);
  window.console.warn = (...a) => { warns.push(a.map(String).join(' ')); };
  const unhandled = [];
  window.addEventListener('unhandledrejection', (e) => unhandled.push(String(e.reason)));

  const bodyHtml = /<body[^>]*>([\s\S]*?)<\/body>/.exec(html)?.[1] ?? '';
  window.document.body.innerHTML = bodyHtml;
  window.eval(js);

  return { window, document: window.document, sentFrames, infos, warns, unhandled, dom, dims };
}

// 等终端完成握手（WELCOME 处理完）：shell prompt 非空白字符出现即 OUTPUT 流通。
// 必须 trim：xterm 挂载后空行占位使 textContent 非零，不 trim 则 waitReady 退化为
// 恒真、全部后续断言抢跑 WELCOME（实测：title 未置位/infos 空/输入门未生效全链假失败）
async function waitReady(document) {
  await waitFor(() => (document.querySelector('.xterm-rows')?.textContent ?? '').trim().length > 0, '终端首输出');
}

// 向终端注入文本（phase04-dom.mjs typeText 同形态：InputEvent 链 → onData → INPUT 帧）
function typeText(window, text) {
  const ta = window.document.querySelector('.xterm-helper-textarea');
  ta.focus();
  const lines = text.split('\n');
  lines.forEach((line, i) => {
    if (line.length > 0) {
      ta.dispatchEvent(new window.InputEvent('input', { data: line, inputType: 'insertText', bubbles: true, cancelable: true }));
    }
    if (i < lines.length - 1) {
      ta.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Enter', keyCode: 13, which: 13, bubbles: true, cancelable: true }));
    }
  });
}

const panel = (document) => ({
  visible: !document.getElementById('status')?.hidden,
  title: document.getElementById('status-title')?.textContent ?? '',
  body: document.getElementById('status-body')?.textContent ?? '',
  hint: document.getElementById('status-hint')?.textContent ?? '',
  reloadLink: document.querySelector('#status-hint a')?.textContent ?? '',
});

// 统一清理：先杀服务端让 WS 断开落定，再关 jsdom（phase04-dom.mjs 顺序纪律）
async function cleanup(ctx, srv) {
  try { srv.kill(); } catch { /* 已退出忽略 */ }
  await sleep(150);
  try { ctx.dom.window.close(); } catch { /* 忽略 */ }
}

// ═══════════════════ D1：ro 形态三要素（05-UAT.md Test 3 逻辑面） ═══════════════════
async function d1RoModeGates() {
  console.log('D1: ro 形态三要素（[ro] 标题前缀 / 键盘不可输入 / resize 零上行 / console 一次性提示）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port }, { path: pathFromUrl(inst.shareRO) });
  try {
    await waitReady(ctx.document);
    // ① 标题 [ro] 前缀恒最前（remoteTitle 默认 'wesh'——bash --norc 不发 OSC 标题）
    check('D1a', 'ro 端标题 [ro] 前缀恒最前',
      ctx.document.title.startsWith('[ro] '), `title=${JSON.stringify(ctx.document.title)}`);
    // ② console 一次性 read-only 提示（三要素：输入不发送/可能裁剪/reattach 恢复）
    check('D1b', 'console 一次性 read-only 提示（三要素齐备且仅一次）',
      ctx.infos.length === 1 && /input is not sent/.test(ctx.infos[0]) && /clipped/.test(ctx.infos[0]) && /reattach/.test(ctx.infos[0]),
      `infos=${ctx.infos.length} 条`);
    // ③ 键盘不可输入：typeText 后零 INPUT 上行（disableStdin 门）
    ctx.sentFrames.length = 0;
    typeText(ctx.window, 'echo SHOULD_NOT_SEND\n');
    await sleep(400);
    check('D1c', 'ro 端键盘输入零 INPUT 上行帧',
      !ctx.sentFrames.includes(INPUT), `上行帧=[${ctx.sentFrames.join(',')}]`);
    // ④ 窗口 resize 零 RESIZE 上行（D-09 第一闸）
    ctx.sentFrames.length = 0;
    ctx.dims.w = 900; ctx.dims.h = 500;
    ctx.window.dispatchEvent(new ctx.window.Event('resize'));
    await sleep(500); // 100ms debounce + fit + onResize 链路落定
    check('D1d', 'ro 端窗口 resize 零 RESIZE 上行帧（D-09 前端闸）',
      !ctx.sentFrames.includes(RESIZE), `上行帧=[${ctx.sentFrames.join(',')}]`);
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D2：递补升格 UX（05-UAT.md Test 4 逻辑面） ═══════════════════
async function d2PromotionUx() {
  console.log('D2: 递补升格（ro 旁观 → owner 断开 → 前缀消失 + 键盘激活 + 零新 UI 组件）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  // owner 占位：原生 WS 直拨（无认证模式 owner 空位先到先得）
  const owner = await dialHello(inst.port);
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port }, { path: pathFromUrl(inst.shareRW) });
  try {
    await waitReady(ctx.document);
    // 旁观期：rw 链接第二端降级 ro（D-06/D-07）
    check('D2a', 'owner 在位时 rw 链接第二端降级旁观（[ro] 前缀 + 输入不发送）',
      ctx.document.title.startsWith('[ro] ') && ctx.infos.length === 1,
      `title=${JSON.stringify(ctx.document.title)} infos=${ctx.infos.length}`);

    // owner 断开 → 升格
    owner.ws.close();
    await waitFor(() => !ctx.document.title.startsWith('[ro] '), '升格后标题前缀移除', 8000);
    check('D2b', 'owner 断开后标题 [ro] 前缀消失（升格信号，D-07 零新 UI 纪律）',
      true, `title=${JSON.stringify(ctx.document.title)}`);

    // 升格后键盘激活：typeText → INPUT 上行帧出现
    ctx.sentFrames.length = 0;
    typeText(ctx.window, 'echo promoted\n');
    await waitFor(() => ctx.sentFrames.includes(INPUT), '升格后 INPUT 上行', 3000);
    check('D2c', '升格后键盘激活（INPUT 帧恢复上行）', true, 'INPUT 已上行');

    // 全程零 toast/badge/通知组件（D-07：模式指示只有标题前缀 + disableStdin）
    const junk = ctx.document.querySelector('[class*="toast"],[id*="toast"],[class*="badge"],[id*="badge"],[class*="notif"],[id*="notif"]');
    check('D2d', '全程零 toast/badge/通知组件',
      junk === null && ctx.infos.length === 1, `junk节点=${junk !== null} infos=${ctx.infos.length}`);
  } finally {
    try { owner.ws.close(); } catch { /* 已关闭 */ }
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D3：503 专版（05-UAT.md Test 6 逻辑面） ═══════════════════
async function d3ServerFullPanel() {
  console.log('D3: 503 专版（Server is full 面板 + 槽位释放后重新进入）');
  const inst = await startWesh(['--writable', '--max-clients', '1', '--', 'bash', '--norc', '--noprofile']);
  // 首客户端占槽
  const ticket = await attachTicket(inst.port, tokenFromUrl(inst.shareRO));
  const occupant = await dialHello(inst.port, { ticket });
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port }, { path: pathFromUrl(inst.shareRO) });
  try {
    // 第二端打开 ro 链接 → /api/attach 早闸 503 → 专版面板
    await waitFor(() => panel(ctx.document).visible, '503 面板出现');
    const p = panel(ctx.document);
    check('D3a', '满员时第二端显示 Server is full 专版（文案三件套 + Reload 链接）',
      p.title === 'Server is full'
      && /maximum number of attached clients/.test(p.body)
      && /Wait for a slot to free up/.test(p.hint)
      && p.reloadLink === 'Reload this page',
      `title=${JSON.stringify(p.title)} hint=${JSON.stringify(p.hint)}`);
    // 面板态无终端画面（早闸于 /api/attach，WS 从未建连——无 HELLO 上行）
    check('D3b', '503 面板态 WS 从未建连（早闸，无 HELLO 上行）',
      !ctx.sentFrames.includes(HELLO), `上行帧=[${ctx.sentFrames.join(',')}]`);
  } finally {
    await cleanup(ctx, { kill: () => {} }); // 先关页面，实例留给下段
  }
  // 槽位释放后刷新重新进入（新开 jsdom 页 = 手动刷新的等价物）
  occupant.ws.close();
  await sleep(300);
  const ctx2 = await loadTerminal({ scheme: inst.scheme, port: inst.port }, { path: pathFromUrl(inst.shareRO) });
  try {
    await waitReady(ctx2.document);
    check('D3c', '首客户端断开后刷新第二端可进入', true, 'attach 成功');
  } finally {
    await cleanup(ctx2, inst);
  }
}

// ═══════════════════ D4：无效链接专版（05-UAT.md Test 7 逻辑面） ═══════════════════
// C-3 面板的真实可达场景（UI-SPEC §Copywriting 矩阵 "401 携 token" 行）：
// 旁观者经 token 链接进入（GET 200，从未走 Basic）→ wesh 重启 token 全废 →
// 页面内重连 POST 携旧 token → 服务端无 Basic 缓存可委托 → 401 → C-3 专版
// （"regenerated each time wesh restarts" 文案正对应此场景）。jsdom 以
// 「不注入 Authorization 头」等效"浏览器无 Basic 缓存"形态。
// 对照：operator 通道（Basic 通过）携错 token → 委托原链签发（D-01/R-05 既定，
// 非 C-3 面）——该形态由 phase05.mjs S4 的 401 形状断言与本文件 D4a 夹逼覆盖。
async function d4InvalidShareLink() {
  console.log('D4: 无效链接（错 token → 401 → Invalid share link 专版，凭据无缓存 + 无认证双形态）');
  // ① 凭据模式、无 Basic 缓存（旁观者重启失效场景）：POST 携错 token → 401 → C-3
  const inst1 = await startWesh(['--credential', UAT_CREDENTIAL, '--writable', '--', 'bash', '--norc', '--noprofile']);
  const badPath = pathFromUrl(inst1.shareRO).replace(tokenFromUrl(inst1.shareRO), 'uatBADTOKEN00');
  const ctx1 = await loadTerminal({ scheme: inst1.scheme, port: inst1.port }, { path: badPath });
  try {
    await waitFor(() => panel(ctx1.document).visible, 'Invalid share link 面板（凭据模式无缓存）', 10000);
    const p = panel(ctx1.document);
    check('D4a', '凭据模式错 token（无 Basic 缓存）→ Invalid share link 专版（三件套）',
      p.title === 'Invalid share link'
      && /invalid or has expired/.test(p.body)
      && /regenerated each time wesh restarts/.test(p.body)
      && /Ask the operator for a new link/.test(p.hint),
      `title=${JSON.stringify(p.title)}`);
  } finally {
    await cleanup(ctx1, inst1);
  }
  // ② 无认证模式：错 token → 401 → C-3 专版（G-05-7，用户 2026-08-22 裁决：
  // 不弹登录框的通道必须弹 Invalid 面板——服务端携错 token 改返 401，
  // 前端「携 token 401 → C-3」既有分支承接，零前端改动）
  const inst2 = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const badPath2 = pathFromUrl(inst2.shareRO).replace(tokenFromUrl(inst2.shareRO), 'uatBADTOKEN00');
  const ctx2 = await loadTerminal({ scheme: inst2.scheme, port: inst2.port }, { path: badPath2 });
  try {
    await waitFor(() => panel(ctx2.document).visible, 'Invalid share link 面板（无认证）', 10000);
    check('D4b', '无认证实例错 token → 401 → Invalid share link 专版（G-05-7）',
      panel(ctx2.document).title === 'Invalid share link', `title=${JSON.stringify(panel(ctx2.document).title)}`);
  } finally {
    await cleanup(ctx2, inst2);
  }
}

// ═══════════════════ D5：1013 专版 + 手动刷新链路（05-UAT.md Test 5 逻辑面） ═══════════════════
async function d5SlowConsumerPanel() {
  console.log('D5: 1013 专版（事件循环阻塞制造 stall → Disconnected 面板 → 刷新重连最新输出）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  // 被测端：jsdom 页开 ro 链接
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port }, { path: pathFromUrl(inst.shareRO) });
  // 驱动端：独立子进程（rw owner 制造输出洪水）——独立事件循环不随主进程阻塞 stall
  const driver = spawn(process.execPath, [new URL('./phase05-flood-driver.mjs', import.meta.url).pathname, String(inst.port)], { stdio: ['ignore', 'pipe', 'pipe'] });
  let driverOut = '';
  driver.stdout.on('data', (d) => { driverOut += d.toString(); });
  driver.stderr.on('data', () => {});
  try {
    await waitReady(ctx.document);
    // 等洪水起流
    await waitFor(() => driverOut.includes('FLOOD_STARTED'), 'driver 洪水起流');
    // 分段阻塞事件循环：每段 200ms、段间让出循环处理 stderr 事件并检查踢出标记。
    // 踢出后立刻解除阻塞——1013 关闭帧写出带 5s 超时（multi_test.go:966 登记），
    // 整段长阻塞会让超时先烧完、关闭帧永不上 wire（客户端只见 1006，首版实测）
    let kicked = false;
    const t0 = Date.now();
    while (!kicked && Date.now() - t0 < 15000) {
      Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 200);
      await sleep(10); // 让出循环：stderr data 事件落盘
      // 08-05 迁移：kick 检测 = detach 事件 reason=kick code=1013（D-21 折入形态——
      // 原 'slow_consumer' 子串消费已随独立事件行删除而失效）
      kicked = parseEvents(inst.stderrText()).some((m) => m.event === 'detach' && m.reason === 'kick' && m.code === 1013);
    }
    if (!kicked) throw new Error('15s 内未见 detach reason=kick 踢出事件（stall 夹具失效）');
    // 解除阻塞：内核缓冲排干 → outbox 队列尾部 1013 关闭帧到达 → onclose(1013) → C-1 专版
    await waitFor(() => panel(ctx.document).visible, '1013 面板出现', 15000);
    const p = panel(ctx.document);
    check('D5a', 'stall 端被踢后显示 Disconnected 专版（C-1 文案 + Reload 链接）',
      p.title === 'Disconnected'
      && /could not keep up with the session output/.test(p.body)
      && /To reattach from the latest output/.test(p.hint)
      && p.reloadLink === 'Reload this page',
      `title=${JSON.stringify(p.title)}`);
    // 其他端不受影响：driver 子进程持续收流（软轮询等字节累积越阈——500ms 进度
    // 行首行未至时硬读会得 0 的实测竞态）且未被关闭
    let lastBytes = 0;
    const t2 = Date.now();
    while (Date.now() - t2 < 8000 && lastBytes <= 5_000_000) {
      lastBytes = Number(/BYTES (\d+)(?!.*BYTES)/s.exec(driverOut)?.[1] ?? 0);
      if (lastBytes > 5_000_000) break;
      await sleep(100);
    }
    check('D5b', '其他端不受影响（driver 持续收流且连接存活）',
      lastBytes > 5_000_000 && !driverOut.includes('CLOSED'),
      `driver已收=${lastBytes} 被关=${driverOut.includes('CLOSED')}`);
  } finally {
    try { driver.kill('SIGKILL'); } catch { /* 忽略 */ }
    await cleanup(ctx, { kill: () => {} }); // 实例留给刷新段
  }
  // 手动刷新链路：新开 jsdom 页凭原 ro URL 重新 attach——从最新输出看起（无全量回放，
  // D-12 drain 语义：若全量回放 20MB 洪水可见文本量必然巨大）
  const ctx2 = await loadTerminal({ scheme: inst.scheme, port: inst.port }, { path: pathFromUrl(inst.shareRO) });
  try {
    await waitReady(ctx2.document);
    await sleep(500);
    const bytes = (ctx2.document.querySelector('.xterm-rows')?.textContent ?? '').trim().length;
    check('D5c', '刷新后凭原 URL 重新 attach 成功且从最新输出看起（无历史洪水回放）',
      bytes > 0 && bytes < 100000, `可见文本量=${bytes}`);
  } finally {
    await cleanup(ctx2, inst);
  }
}

// ═══════════════════ D6：G-05-1 视口约束渲染（05-11 前端半侧行为面：约束 rows / 会话 cols 折行 / 升格解除） ═══════════════════
// 场景装配复刻 G-05-1 用户实测形态：窄 owner（40x10，原生 WS 配角）把会话尺寸钉在 40x10；
// jsdom 页经 rw 链接第二端进入降级 ro——宽端旁观（布局桩 720x408 → fit 80x24）。
// D6b 机理登记：readline/printf 输出按 PTY 40 列生成环绕点，前端约束 40 列渲染 =
// 换行点一致 = 不叠写（宽端无约束时 80 个 'A' 单行呈现，断言必败——区分度锁定）。
async function d6ViewportConstraint() {
  console.log('D6: 视口约束渲染（约束 rows=会话 rows / 长行在会话 cols 折行 / 升格解除约束回窗口尺寸）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  // 窄 owner 配角：40x10 钉住会话尺寸（owner 模式参与集单员）
  const owner = await dialHello(inst.port, { cols: 40, rows: 10 });
  // 宽端旁观页：rw 链接第二端降级 ro（D-06/D-07）——G-05-1 用户场景的大窗口 B
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port }, { path: pathFromUrl(inst.shareRW) });
  try {
    await waitReady(ctx.document);
    // D6a：约束 rows——DOM 渲染器行 div 数恒等于 term.rows；会话 rows=10 约束生效
    //（无约束时是 fit 的 24，区分度在场；phase04-dom.mjs:387 `.xterm-rows > div` 先例通道）
    check('D6a', '宽端 ro 旁观 xterm 渲染行数被约束到会话 rows（.xterm-rows 行数 = 10 而非 24）',
      ctx.document.querySelector('.xterm-rows').childElementCount === 10,
      `行数=${ctx.document.querySelector('.xterm-rows').childElementCount}`);

    // D6b：会话 cols 折行（叠写回归的 DOM 层等价物）——owner 注入 80 个 'A'
    //（恰两倍会话列宽；bash --norc 支持花括号展开，printf 'A%.0s' 每参数字出一 'A'）；
    // 约束渲染下折行点 = 会话 cols=40：存在恰 40 个 'A' 的行且相邻下一行同恰 40 个 'A'
    owner.ws.send(concat(new Uint8Array([INPUT]), enc.encode(`printf 'A%.0s' {1..80}\n`)));
    const rows = await waitFor(() => {
      const divs = [...ctx.document.querySelectorAll('.xterm-rows > div')];
      const i = divs.findIndex((d) => d.textContent === 'A'.repeat(40));
      if (i !== -1 && divs[i + 1]?.textContent === 'A'.repeat(40)) return divs;
      return null;
    }, '80 个 A 在会话 cols=40 处折行为相邻两 div 各 40 字符');
    const ai = rows.findIndex((d) => d.textContent === 'A'.repeat(40));
    check('D6b', '长行输出在会话 cols 处折行（恰 40+40 两相邻行，而非宽端窗口 80 单列一行）',
      ai !== -1 && rows[ai + 1]?.textContent === 'A'.repeat(40),
      `折行行号=${ai} 下一行同长=${rows[ai + 1]?.textContent === 'A'.repeat(40)}`);

    // D6c：升格解除约束（G-05-1 设计约束 4 端到端）——owner 断开 → 本页升格 rw
    //（升格 Welcome 携 cand.dims = 本页 Hello 登记 80x24 → sessionDims 更新 → refit
    // 解除约束）；waitFor 轮询吸纳升格 Welcome 与紧随 recalcNow 推送双帧相继到达（同值幂等）
    owner.ws.close();
    await waitFor(() => !ctx.document.title.startsWith('[ro] '), '升格后标题前缀移除', 8000);
    await waitFor(() => ctx.document.querySelector('.xterm-rows').childElementCount === 24, '约束解除渲染回窗口 24 行', 5000);
    check('D6c', 'owner 断开升格后约束解除（[ro] 前缀消失 + 渲染回窗口尺寸 24 行）',
      true, `title=${JSON.stringify(ctx.document.title)} 行数=${ctx.document.querySelector('.xterm-rows').childElementCount}`);
  } finally {
    try { owner.ws.close(); } catch { /* 已关闭 */ }
    await cleanup(ctx, inst);
  }
}

const scenarios = [d1RoModeGates, d2PromotionUx, d3ServerFullPanel, d4InvalidShareLink, d5SlowConsumerPanel, d6ViewportConstraint];
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
const passedN = results.filter((r) => r.ok === true).length;
const failedN = results.filter((r) => r.ok === false).length;
console.log(`\n结果: ${passedN}/${results.length} DOM 断言通过${failed ? `，${failed} 个场景异常` : ''}`);
process.exit(failedN === 0 && failed === 0 ? 0 : 1);
