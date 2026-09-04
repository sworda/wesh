// Phase 12 PC-06 前端 reset 全链逻辑面自动化 UAT（jsdom + 真实 per-client wesh 实例）。
//
// 定位：12-01 Task 3——Welcome 模式位（D-08）→ 前端 terminal.reset()（D-09）全链的
// 可执行断言。jsdom 加载真实构建产物 web/dist/index.html，SpyWebSocket 连接真实
// spawn 的 per-client wesh 实例端到端断言（headless 硬约束，根 CODEBUDDY.md 分层
// 策略层 3；Playwright 浏览器观感层归 Phase 14 herdr 全链，D-13 防重复建设红线）。
// 协议层 Welcome.session 对照（shared 模式取值）由 12-04 phase12.mjs S1 承担。
//
// reset ⊇ clear 的判别通道（@xterm/headless 探针实证，2026-09-04）：
//   - term.clear()：清视口滚动行，但【不退出 alternate screen】且【不清其背后的
//     normal buffer】——旧会话若停在全屏程序（less/vim/tmux 的 1049h）内断线，
//     重连后新会话写入仍落在 alt buffer；一旦 1049l 弹回，normal buffer 的旧屏
//     残影复活。
//   - term.reset()：buffer 全清 + 终端状态复位（alt screen 退出、modes 复位）。
//   - 由此 D1/D3 以「旧会话先进 alt screen 藏好 normal buffer 残影 → 重连 → 新会话
//     1049l 弹回」链路观测 reset 是否发生（DOM 通道 phase06-dom.mjs D1h 同族）。
//
// 场景：
//   D1 per-client 1006 重连全链 → 新 WELCOME（session:"per-client"）→ reset 生效：
//      旧 normal buffer 残影不复活 + 新会话内容完整 + 静默零新面板（D-09/D-10）
//   D2 per-client ro 端 window resize → RESIZE（0x31）真实上行（D-07 第一闸按模式位
//      放开 + D-06 服务端直通配对）；shared ro 对照同操作全程零 RESIZE（05-08
//      shared 语义逐字保留，prohibition 回归锁）
//   D3 缺 session 键 Welcome（旧服务端形态，SpyWebSocket 投递拦截剥键注入）→
//      不 reset：旧 normal buffer 内容保持在场（D-08 防御性缺省，误 reset 即
//      CORE-05 接回同进程语义下清掉有效旧屏 = 数据面破坏）
//
// 夹具（phase06-dom.mjs loadTerminal/SpyWebSocket 形态逐字复用 + 一件延伸）：
//   - SpyWebSocket 投递拦截面（D3 旧服务端形态注入通道）：opts.stripSession 时
//     包装 bundle 经 onmessage 赋值的处理器——投递前改写 WELCOME 帧，剥除 session
//     键（Node 24 WebSocket.prototype.onmessage 为原型访问器，子类 setter 包装 +
//     super 转发；bundle 处理器只读 ev.data，包装投递 {data} 等价）。
//
// 隔离纪律（phase06-dom.mjs:38 同款）：每场景独立 spawn 实例 + 独立 jsdom。
//
// 红线（phase04.mjs:6-9 / phase06-dom.mjs:40-41 纪律逐字沿用）：token/凭据/pid 值
// 永不进 check detail/控制台输出/汇总行——detail 只打状态码/布尔/形状/标记常量；
// assertOutputClean() 运行时自净断言兜底（review #7）。
//
// 运行：node web/uat/phase12-dom.mjs [wesh 二进制路径]（默认 /tmp/wesh-uat/wesh）
import { spawn } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { JSDOM } from 'jsdom';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';
const DIST = new URL('../dist/index.html', import.meta.url).pathname;

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57;
const SUBPROTOCOL = 'wesh.v1';

const enc = new TextEncoder();
const dec = new TextDecoder();

const results = [];
// 全部已发 detail 收集（assertOutputClean 遍历材料——review #7 运行时自净断言）
const emittedDetails = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  emittedDetails.push(String(detail));
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
// 平台豁免记录形态：不计失败（headless 硬约束，CODEBUDDY.md 显式豁免条款）
const skip = (id, name, reason) => {
  results.push({ id, name, ok: null });
  emittedDetails.push(String(reason));
  console.log(`  SKIP  ${id} ${name} — ${reason}`);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function waitFor(fn, label, timeout = 5000) {
  const t0 = Date.now();
  for (;;) {
    try { const v = fn(); if (v) return v; } catch { /* 断言中途态忽略 */ }
    if (Date.now() - t0 > timeout) throw new Error(`waitFor 超时: ${label}`);
    await sleep(25);
  }
}

// SpyWebSocket 构造计数（模块级，构造器内递增）——重连链时序断言材料；各场景以
// 基线快照取相对值
let constructed = 0;
// startWesh 解析 stdout 时把分享链接 token 留入本闭包数组（只作 assertOutputClean
// 断言材料）——红线：token 值永不进 check detail/控制台输出/汇总行
const sensitiveTokens = [];

// 分享链接 URL → token（/s/{token}/ 路径段；值只作 assertOutputClean 断言材料——红线）
const tokenFromUrl = (url) => /\/s\/([^/]+)\//.exec(url)[1];

// 启动 wesh 实例（phase06-dom.mjs startWesh 同形态：--bind 127.0.0.1 --port 0 +
// stdout 解析 + stderr 捕获 + child 句柄 + SIGKILL kill）。本脚本场景 plain '/'
// 进入（无认证模式，/api/attach 404 探测直连链路既有），分享链接行不用于导航。
// 12-01 场景以 --session-mode per-client 装配（PC-06 前提）。
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
          // token 留入模块级闭包（assertOutputClean 唯一消费点——值不进任何输出）
          for (const link of [shareRO, shareRW]) {
            if (link !== null) sensitiveTokens.push(tokenFromUrl(link));
          }
          resolve({ port: Number(m[2]), scheme: m[1], shareRO, shareRW, stderrText: () => stderr, kill: () => child.kill('SIGKILL'), child });
        }, 50);
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

// 加载 dist 到 jsdom 并执行 bundle（phase06-dom.mjs loadTerminal 形态逐字复用 +
// SpyWebSocket 投递拦截一件延伸）。全场景无认证形态（plain '/' 进入；服务端无认证
// 模式 /api/attach 404 → 前端跳过 ticket 直连 WS）。
//
// 延伸面（D3 注入通道）：opts.stripSession=true 时，bundle 经 onmessage 赋值注册的
// 处理器被包装——服务端 WELCOME 帧投递前剥除 session 键（旧服务端 wire 形态），
// 其余帧原样投递。bundle WELCOME 处理器只读 ev.data（main.ts onmessage 首行
// new Uint8Array(ev.data)），改写帧后以 {data} 普通对象投递等价。
async function loadTerminal(srv, opts = {}) {
  const html = readFileSync(DIST, 'utf8');
  const js = /<script type="module" crossorigin>([\s\S]*?)<\/script>/.exec(html)[1];
  const origin = `${srv.scheme}://${srv.host ?? '127.0.0.1'}:${srv.port}`;
  const url = `${origin}${opts.path ?? '/'}`;
  const dom = new JSDOM('', { url, pretendToBeVisual: true, runScripts: 'outside-only' });
  const { window } = dom;

  // ── 平台能力注入/桩 ──
  // SpyWebSocket：记录全部上行帧首字节 + 合成 CloseEvent 能力 + 实例台账 + 投递拦截
  const sentFrames = [];
  const sockets = [];
  // 合成关闭事件构造（A2：jsdom 25 CloseEvent 构造器实测可用；构造受阻回退
  // Event + code 赋值——回退形态存在即 A2 登记兜底）
  const makeCloseEvent = (code) => {
    try {
      return new window.CloseEvent('close', { code });
    } catch {
      return Object.assign(new window.Event('close'), { code });
    }
  };
  // D3 旧服务端形态注入：WELCOME 帧 session 键剥除（缺键 = 旧服务端，D-08 识别
  // 契约的注入面）。非 WELCOME 帧 / 已无键 / 畸形 JSON 原样投递（防御形态）
  const stripSession = opts.stripSession === true;
  const stripSessionFromEvent = (ev) => {
    try {
      const u8 = new Uint8Array(ev.data);
      if (u8[0] !== WELCOME) return ev;
      const w = JSON.parse(dec.decode(u8.subarray(1)));
      if (!('session' in w)) return ev;
      delete w.session;
      const payload = enc.encode(JSON.stringify(w));
      const frame = new Uint8Array(1 + payload.length);
      frame[0] = WELCOME;
      frame.set(payload, 1);
      return { data: frame.buffer };
    } catch {
      return ev; // 畸形帧原样（WELCOME 分支自身的 try/catch 承担丢弃语义）
    }
  };
  window.WebSocket = class extends WebSocket {
    constructor(url, ...a) {
      super(url, ...a);
      constructed++; // 模块级构造计数——重连链时序断言材料
      this.binaryType = 'arraybuffer';
      this._savedClose = null; // synthClose 留存的 onclose 处理器副本
      this._onMsgOrig = null; // 投递拦截前的原处理器（get 返回值保持一致）
      sockets.push(this);
    }
    send(data) {
      const u8 = data instanceof ArrayBuffer ? new Uint8Array(data) : new Uint8Array(data.buffer ?? data);
      sentFrames.push(u8[0]);
      return super.send(data);
    }
    // 合成关闭：取存处理器并置 null（抑制随后真实 close 的 1000 事件混入断言面）→
    // 真实底层连接尽力关闭 → 以处理器调用合成事件（同步驱动 bundle onclose 分派）
    synthClose(code) {
      const handler = this.onclose;
      this.onclose = null;
      this._savedClose = handler;
      try { this.close(); } catch { /* 已关闭忽略 */ }
      if (handler !== null && handler !== undefined) handler.call(this, makeCloseEvent(code));
    }
    // 投递拦截面（D3）：bundle 经 onmessage 赋值注册处理器（main.ts 唯一注册形态）
    // ——stripSession 时包装改写后投递。Node 24 的 WebSocket.prototype.onmessage
    // 为原型访问器，子类 setter 经 super 转发底层注册（事件派发路径不变）；
    // get 返回原处理器（synthClose 同族的读写一致性，本脚本无读方）
    set onmessage(handler) {
      this._onMsgOrig = handler ?? null;
      if (stripSession && typeof handler === 'function') {
        super.onmessage = (ev) => handler.call(this, stripSessionFromEvent(ev));
      } else {
        super.onmessage = handler;
      }
    }
    get onmessage() {
      return this._onMsgOrig;
    }
  };
  // CR-01 D10 夹具（phase06-dom.mjs 同形态保留）：hold 第 holdN 次 fetch 的 resolve
  const holdN = opts.holdAttachFetchN ?? 0;
  let fetchSeq = 0;
  let releaseHeld = null;
  window.fetch = (u, o) => {
    const p = fetch(new URL(u, origin), o);
    fetchSeq++;
    if (fetchSeq === holdN) {
      return new Promise((resolve) => { releaseHeld = () => resolve(p); });
    }
    return p;
  };
  window.TextEncoder = TextEncoder;
  window.TextDecoder = TextDecoder;
  window.matchMedia = (q) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, onchange: null, dispatchEvent: () => false });
  // beforeunload 记账：包装转发原实现（行为零漂移），on/off 计数供 D1 重连断言
  const bu = { on: 0, off: 0 };
  const origAdd = window.addEventListener.bind(window);
  const origRemove = window.removeEventListener.bind(window);
  window.addEventListener = (type, listener, ...rest) => {
    if (type === 'beforeunload') bu.on++;
    return origAdd(type, listener, ...rest);
  };
  window.removeEventListener = (type, listener, ...rest) => {
    if (type === 'beforeunload') bu.off++;
    return origRemove(type, listener, ...rest);
  };
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

  // 观测钩子：console 捕获与异常（调试通道）
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

  // 代际守卫场景二次驱动（phase06-dom.mjs 同形态保留）
  const staleClose = (inst, code) => {
    inst._savedClose?.call(inst, makeCloseEvent(code));
  };

  return { window, document: window.document, sentFrames, infos, warns, unhandled, dom, dims, sockets, bu, staleClose, releaseHeldFetch: () => releaseHeld?.() };
}

// 等终端完成握手（WELCOME 处理完）：shell prompt 非空白字符出现即 OUTPUT 流通
//（phase05-dom.mjs waitReady 同形态——必须 trim，空行占位不 trim 则抢跑 WELCOME）
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

// 终端 DOM 可见文本（清屏断言通道——term.clear()/reset() 后 buffer 行重渲染，
// 异步由 waitFor 吸纳；phase06-dom.mjs:287-288 D1h 同通道）
const terminalText = (document) => document.querySelector('.xterm-rows')?.textContent ?? '';

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

// ═══════════════════ D1：per-client 重连 reset 全链（PC-06 主干，D-08/D-09/D-10） ═══════════════════
// 链路：旧会话 normal buffer 写残影标记 → 1049h 进 alt screen（残影藏身处——
// clear() 不退出 alt screen 且不清背后的 normal buffer）→ 1006 断连 → 重连
// （per-client = 全新进程）→ 新 WELCOME（session:"per-client"）→ reset 生效 →
// 新会话 1049l 弹回 → 断言 normal buffer 旧残影不复活 + 新内容完整 + 静默零新面板。
// 对照证据（@xterm/headless 探针）：无 reset 时 1049l 弹回 normal buffer 残影复活。
async function d1PerClientResetChain() {
  console.log('D1: per-client 1006 重连全链 → WELCOME session=per-client → terminal.reset()（旧屏残影清除，D-09/D-10）');
  const inst = await startWesh(['--session-mode', 'per-client', '--writable', '--', 'bash', '--norc', '--noprofile']);
  const base = constructed;
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
  try {
    await waitReady(ctx.document);
    check('D1a', '首连会话建立（面板隐藏 + 构造 +1）',
      !panel(ctx.document).visible && constructed === base + 1,
      `构造=${constructed - base} 面板可见=${panel(ctx.document).visible}`);
    // 旧会话 normal buffer 写入残影对照标记（真实链路：typeText → INPUT → PTY →
    // echo 输出 → OUTPUT → 终端 DOM）
    typeText(ctx.window, 'echo NORMGHOST7\n');
    await waitFor(() => terminalText(ctx.document).includes('NORMGHOST7'), 'NORMGHOST7 经 echo 写入正常 buffer');
    // 进 alternate screen（printf 输出 1049h 真实到达）——残影藏于 normal buffer
    typeText(ctx.window, "printf '\\033[?1049h'\n");
    await waitFor(() => !terminalText(ctx.document).includes('NORMGHOST7'), 'alt screen 激活（normal buffer 内容隐藏）');
    typeText(ctx.window, 'echo ALTLIVE7\n');
    await waitFor(() => terminalText(ctx.document).includes('ALTLIVE7'), 'ALTLIVE7 写入 alt buffer');
    check('D1b', 'alt screen 场景就位（残影藏于 normal buffer——clear() 不触达处）', true, '1049h 已进 alt screen');

    // 合成 1006 异常断开 → 退避自动重连 → per-client 重连 = 全新进程 + 新 WELCOME
    ctx.sockets[ctx.sockets.length - 1].synthClose(1006);
    await waitFor(() => constructed === base + 2, '退避后自动重连（构造计数 +1）', 3000);
    await waitFor(() => ctx.bu.on === 2, '新 WELCOME 处理完成（beforeunload 重注册，D1i 通道）', 3000);
    // 新会话写入新标记——reset 已生效则落在干净 normal buffer
    typeText(ctx.window, 'echo D1NEW7\n');
    await waitFor(() => terminalText(ctx.document).includes('D1NEW7'), '新会话输出到达');
    // 新会话内 1049l 弹回：reset 已生效则本就在 normal buffer（无可见变化）；
    // 未 reset 则弹回仍持有旧内容的 normal buffer——残影复活（探针 Case A 形态）
    typeText(ctx.window, "printf '\\033[?1049l'; echo EXITED7\n");
    await waitFor(() => terminalText(ctx.document).includes('EXITED7'), '1049l 命令执行完成（输出有序，弹回已先行）');
    check('D1c', 'reset 证据一：旧 alt buffer 内容（ALTLIVE7）不残留 DOM',
      !terminalText(ctx.document).includes('ALTLIVE7'), `ALTLIVE7 在场=${terminalText(ctx.document).includes('ALTLIVE7')}`);
    check('D1d', 'reset 证据二：1049l 弹回后旧 normal buffer 残影（NORMGHOST7）不复活（reset 清 buffer + 退 alt screen；仅 clear() 时残影复活——探针 Case A）',
      !terminalText(ctx.document).includes('NORMGHOST7'), `NORMGHOST7 在场=${terminalText(ctx.document).includes('NORMGHOST7')}`);
    check('D1e', '新会话内容完整在场（D1NEW7/EXITED7）——reset 只清旧屏不吞新输出',
      terminalText(ctx.document).includes('D1NEW7') && terminalText(ctx.document).includes('EXITED7'), '新会话标记在场');
    check('D1f', '重连 reset 静默零新面板/文案（D-10——无「新会话」提示，面板保持隐藏）',
      !panel(ctx.document).visible, `面板可见=${panel(ctx.document).visible}`);
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D3：缺 session 键 Welcome 不 reset（D-08 防御性缺省） ═══════════════════
// 注入面：SpyWebSocket 投递拦截剥除 WELCOME 帧 session 键（旧服务端 wire 形态）。
// 链路与 D1 同构直到重连：缺键 Welcome → 按 shared 防御性缺省不 reset → 新会话
// 1049l 弹回后旧 normal buffer 内容保持在场。误 reset = 清掉旧服务端同进程的
// 有效旧屏（CORE-05 接回语义）——prohibition「缺 session 键不 reset」回归锁。
async function d3MissingKeyNoReset() {
  console.log('D3: 缺 session 键 Welcome（旧服务端形态，投递拦截剥键）→ 不 reset——旧 buffer 内容保持在场');
  const inst = await startWesh(['--session-mode', 'per-client', '--writable', '--', 'bash', '--norc', '--noprofile']);
  const base = constructed;
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port }, { stripSession: true });
  try {
    await waitReady(ctx.document);
    check('D3a', '首连会话建立（剥键注入下握手正常，构造 +1）',
      !panel(ctx.document).visible && constructed === base + 1,
      `构造=${constructed - base} 面板可见=${panel(ctx.document).visible}`);
    typeText(ctx.window, 'echo NORMKEEP7\n');
    await waitFor(() => terminalText(ctx.document).includes('NORMKEEP7'), 'NORMKEEP7 经 echo 写入正常 buffer');
    typeText(ctx.window, "printf '\\033[?1049h'\n");
    await waitFor(() => !terminalText(ctx.document).includes('NORMKEEP7'), 'alt screen 激活（normal buffer 内容隐藏）');
    // 1006 重连：新 WELCOME 被剥除 session 键 → 前端按旧服务端（shared）处理
    ctx.sockets[ctx.sockets.length - 1].synthClose(1006);
    await waitFor(() => constructed === base + 2, '退避后自动重连（构造计数 +1）', 3000);
    await waitFor(() => ctx.bu.on === 2, '新 WELCOME 处理完成（beforeunload 重注册）', 3000);
    // 新会话 1049l 弹回：未 reset 则旧 normal buffer 内容复活在场（这正是期望——
    // 缺键 = 旧服务端 = shared 语义，误 reset 才会清掉它）
    typeText(ctx.window, "printf '\\033[?1049l'; echo EXITED7\n");
    await waitFor(() => terminalText(ctx.document).includes('EXITED7'), '1049l 命令执行完成');
    check('D3b', '缺 session 键 Welcome 不 reset——旧 normal buffer 内容（NORMKEEP7）保持在场（D-08 防御性缺省；误 reset 即破坏 prohibition）',
      terminalText(ctx.document).includes('NORMKEEP7'), `NORMKEEP7 在场=${terminalText(ctx.document).includes('NORMKEEP7')}`);
    check('D3c', '新会话输出正常到达（EXITED7）——不 reset 不影响新输出流通',
      terminalText(ctx.document).includes('EXITED7'), '新会话标记在场');
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D2：ro 端 RESIZE 第一闸按模式位放开（PC-05 前端半侧，D-06/D-07 配对） ═══════════════════
// 链路（12-02 Task 2）：per-client ro 实例（无 --writable）jsdom 加载 → WELCOME
//（isRO=true + sessionMode="per-client"）→ 突变布局桩尺寸（720x408 → 900x510，
// fit 80x24 → 100x30）→ window resize 事件 → 100ms debounce → refit() →
// sendResize 上行 RESIZE（0x31）。
// 去重闸说明（main.ts onopen）：Hello 载荷即首次尺寸上报且同步 lastReported
//（80x24），WELCOME refit 的等值上报被去重吞掉——握手后基线恒零 RESIZE，只有
// fit 尺寸真实变化（本场景的布局突变）才产生新帧，判别面唯一。
// shared ro 对照：shared 模式实例同操作——isRO && sessionMode==='shared' →
// sendResize 直接 return（05-08 逐字保留），全程零 RESIZE（静默窗形态负向断言）。
// 隔离纪律：每实例独立 spawn + 独立 jsdom（半场一/半场二先后独立装配收口）。
async function d2RoResizeGate() {
  console.log('D2: per-client ro 端 window resize → RESIZE 上行恢复（D-07）；shared ro 对照零 RESIZE（05-08 保留）');
  const RESIZE_SENT = 0x31;
  const countResize = (frames) => frames.filter((b) => b === RESIZE_SENT).length;

  // 半场一：per-client ro——事件驱动 RESIZE 上行（D-07 第一闸放开）
  {
    const inst = await startWesh(['--session-mode', 'per-client', '--', 'bash', '--norc', '--noprofile']);
    const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
    try {
      await waitReady(ctx.document);
      const roNotified = ctx.infos.some((s) => s.includes('read-only mode'));
      check('D2a', 'per-client ro 会话建立（无 --writable → ro 通知在场 = isRO 通路；握手后零 RESIZE——Hello 即首报 + 去重）',
        roNotified && countResize(ctx.sentFrames) === 0,
        `ro通知=${roNotified} RESIZE帧=${countResize(ctx.sentFrames)}`);
      // 布局桩突变（getComputedStyle 闭包引用同一 dims 对象——proposeDimensions
      // 读 #terminal 父容器 computed width/height）：900x510 → fit 100x30
      ctx.dims.w = 900;
      ctx.dims.h = 510;
      ctx.window.dispatchEvent(new ctx.window.Event('resize'));
      await waitFor(() => countResize(ctx.sentFrames) > 0, 'per-client ro resize 事件 → RESIZE（0x31）上行', 3000);
      check('D2b', 'per-client ro 端 resize 事件后 sentFrames 含 RESIZE（0x31）——D-07 第一闸按模式位放开（与服务端 D-06 直通配对生效）',
        countResize(ctx.sentFrames) >= 1, `RESIZE帧=${countResize(ctx.sentFrames)}`);
    } finally {
      await cleanup(ctx, inst);
    }
  }

  // 半场二：shared ro 对照——同操作全程零 RESIZE（05-08 语义逐字保留）
  {
    const inst = await startWesh(['--', 'bash', '--norc', '--noprofile']);
    const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
    try {
      await waitReady(ctx.document);
      const roNotified = ctx.infos.some((s) => s.includes('read-only mode'));
      check('D2c', 'shared ro 会话建立（对照实例，ro 通知在场）', roNotified, `ro通知=${roNotified}`);
      ctx.dims.w = 900;
      ctx.dims.h = 510;
      ctx.window.dispatchEvent(new ctx.window.Event('resize'));
      // 100ms debounce + 余量静默窗（负向断言：窗口内零 RESIZE = 闸门保持关闭）
      await sleep(400);
      check('D2d', 'shared ro 对照：同操作全程零 RESIZE——05-08 ro 不发语义逐字保留（per-client 放开不渗入 shared 路径，prohibition 回归锁）',
        countResize(ctx.sentFrames) === 0, `RESIZE帧=${countResize(ctx.sentFrames)}`);
    } finally {
      await cleanup(ctx, inst);
    }
  }
}

// 输出自净断言（phase06-dom.mjs:732-737 逐字沿用——红线由注释纪律升级为运行时
// 自证）：遍历全部已发 detail，断言不含任一 token 值或 '/s/' 链接形态串；命中即
// FAIL（防未来回归静默破线）。命中时不回显冒犯内容（只打布尔/计数——红线自保）
function assertOutputClean() {
  const leaked = emittedDetails.some((d) =>
    d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t)));
  check('SEC', "输出自净：全部 detail 零 token 值零 '/s/' 链接形态串（红线运行时自证）",
    !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}

const scenarios = [d1PerClientResetChain, d2RoResizeGate, d3MissingKeyNoReset];
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
assertOutputClean();
const skipped = results.filter((r) => r.ok === null).length;
const passedN = results.filter((r) => r.ok === true).length;
const failedN = results.filter((r) => r.ok === false).length;
console.log(`\n结果: ${passedN}/${results.length - skipped} DOM 断言通过${skipped ? `，${skipped} 项 skipped（豁免）` : ''}${failed ? `，${failed} 个场景异常` : ''}`);
process.exit(failedN === 0 && failed === 0 ? 0 : 1);
