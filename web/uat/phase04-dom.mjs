// Phase 4 渲染/平台逻辑面自动化 UAT（jsdom + Node 原生 WebSocket/fetch）。
//
// 定位：覆盖 04-UAT.md T2-T11 的**代码逻辑面**——门控/防抖/条件注册/事件链/
// prefs 应用/协议帧消费，全部在无浏览器 headless 环境断言（根 CODEBUDDY.md
// 四层测试策略第 3 层）。浏览器平台原生行为（权限弹窗/原生 confirm 框/OS 真实
// IME 栈/像素视觉）任何自动化均不可测，不在此声明覆盖。
//
// 驱动方式：jsdom 加载真实构建产物 web/dist/index.html（单文件 IIFE bundle，
// 无 import/export，直接 window.eval 执行），注入 Node 原生 WebSocket/fetch 与
// 固定布局桩（jsdom 无排版引擎），连接真实 spawn 的 wesh 实例端到端断言。
//
// 布局桩约定：终端 720x408 px、字符 9x17 px → 恰 80x24（与 xterm 默认一致），
// 鼠标事件 clientX/Y → cell 换算确定（cell = (floor(x/9), floor(y/17))）。
//
// 运行：node web/uat/phase04-dom.mjs [wesh 二进制路径]（默认 /tmp/wesh-uat/wesh）
import { spawn } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { JSDOM } from 'jsdom';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';
const DIST = new URL('../dist/index.html', import.meta.url).pathname;

const results = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
// 条件轮询：5s 超时。断言目标多为 WS 帧驱动的异步 DOM 变化
async function waitFor(fn, label, timeout = 5000) {
  const t0 = Date.now();
  for (;;) {
    try { const v = fn(); if (v) return v; } catch { /* 断言中途态忽略 */ }
    if (Date.now() - t0 > timeout) throw new Error(`waitFor 超时: ${label}`);
    await sleep(25);
  }
}

function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, args, { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${args.join(' ')}; stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      const m = /listening on (https?):\/\/([^\s]+):(\d+)/.exec(d.toString());
      if (m) {
        clearTimeout(to);
        resolve({ scheme: m[1], host: m[2], port: Number(m[3]), kill: () => child.kill('SIGKILL'), child });
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

// 加载 dist 到 jsdom 并执行 bundle。opts.clipboard: 'mock'|'absent'；opts.query: URL query 串
async function loadTerminal(srv, opts = {}) {
  const html = readFileSync(DIST, 'utf8');
  const js = /<script type="module" crossorigin>([\s\S]*?)<\/script>/.exec(html)[1];
  const url = `${srv.scheme}://${srv.host}:${srv.port}/${opts.query ?? ''}`;
  const dom = new JSDOM('', { url, pretendToBeVisual: true, runScripts: 'outside-only' });
  const { window } = dom;

  // ── 平台能力注入/桩 ──
  window.WebSocket = WebSocket; // Node 原生（Node >= 22）
  const base = url;
  window.fetch = (u, o) => fetch(new URL(u, base), o);
  window.TextEncoder = TextEncoder;
  window.TextDecoder = TextDecoder;
  // jsdom 无 matchMedia——xterm 6 需要
  window.matchMedia = (q) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, onchange: null, dispatchEvent: () => false });
  // 布局桩：固定 720x408 视口、9x17 字符——鼠标坐标→cell 确定
  const RECT = { x: 0, y: 0, top: 0, left: 0, right: 720, bottom: 408, width: 720, height: 408, toJSON: () => ({}) };
  window.HTMLElement.prototype.getBoundingClientRect = function () { return RECT; };
  Object.defineProperty(window.HTMLElement.prototype, 'offsetWidth', { get() { return 720; }, configurable: true });
  Object.defineProperty(window.HTMLElement.prototype, 'offsetHeight', { get() { return 408; }, configurable: true });
  Object.defineProperty(window.HTMLElement.prototype, 'clientWidth', { get() { return 720; }, configurable: true });
  Object.defineProperty(window.HTMLElement.prototype, 'clientHeight', { get() { return 408; }, configurable: true });
  // canvas 2D 桩：xterm CharMeasure 需 measureText 含字体度量（fontBoundingBoxAscent/
  // Descent——bundle 实测构造期即校验，缺则抛 "Required font metrics not supported"）；
  // webgl getContext 返回 null → WebglAddon 构造抛错 → main.ts:106 catch 回落 DOM 渲染器
  const metrics = (s) => ({ width: s.length * 9, fontBoundingBoxAscent: 14, fontBoundingBoxDescent: 3, actualBoundingBoxAscent: 14, actualBoundingBoxDescent: 3 });
  const ctx2d = { measureText: metrics, fillRect() {}, clearRect() {}, getImageData: (x, y, w, h) => ({ data: new Uint8ClampedArray(w * h * 4) }), putImageData() {}, createImageData: () => ({ data: new Uint8ClampedArray(0) }), setTransform() {}, drawImage() {}, save() {}, restore() {}, beginPath() {}, closePath() {}, rect() {}, clip() {}, fill() {}, translate() {}, scale() {}, rotate() {}, arc() {}, fillText() {}, strokeText() {} };
  window.HTMLCanvasElement.prototype.getContext = function (kind) { return kind === '2d' ? ctx2d : null; };
  // xterm 6 CharSizeService 用 OffscreenCanvas（jsdom 无）——同桩补齐，9x17 字符格
  window.OffscreenCanvas = class {
    constructor(w, h) { this.width = w; this.height = h; }
    getContext(kind) { return kind === '2d' ? ctx2d : null; }
  };
  // jsdom getComputedStyle 对未排版属性返回空串——xterm 坐标换算
  // `parseInt(getPropertyValue('padding-left'))` 得 NaN 致选区/链接坐标全 NaN（实测）。
  // 两处的精准兜底（教训：一刀切 '0px' 会让 fit 算出 0 行列把终端缩没）：
  //   padding-* → '0px'（任意元素）；
  //   width/height 且 el=#terminal → 可变 dims（fit addon proposeDimensions 的唯一读取点，
  //   给 720x408 恰合 80x24；T8 改 dims 即模拟窗口 resize）
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

  // 观测钩子
  const warns = [];
  const errors = [];
  const origWarn = window.console.warn.bind(window.console);
  window.console.warn = (...a) => { warns.push(a.map(String).join(' ')); origWarn(...a); };
  const unhandled = [];
  window.addEventListener('unhandledrejection', (e) => unhandled.push(String(e.reason)));
  const openedUrls = [];
  window.open = (u) => { openedUrls.push(u === undefined ? '(about:blank)' : String(u)); return null; };
  let confirmCalls = 0;
  window.confirm = () => { confirmCalls++; return false; };

  // 剪贴板形态
  const clip = { writes: [], reads: 0 };
  if (opts.clipboard === 'mock') {
    const mock = {
      writeText: (t) => { clip.writes.push(t); return Promise.resolve(); },
      readText: () => { clip.reads++; return Promise.resolve(clip.writes.length ? clip.writes[clip.writes.length - 1] : ''); },
    };
    Object.defineProperty(window.navigator, 'clipboard', { value: mock, configurable: true });
  }
  // 'absent'：jsdom navigator 本无 clipboard——clipboardOK 存在性检测天然 false（D-11 形态）

  // bundle 内顶层 `document` 引用 jsdom document——页面骨架由 HTML 提供，先写 body 再 eval
  const bodyHtml = /<body[^>]*>([\s\S]*?)<\/body>/.exec(html)?.[1] ?? '';
  window.document.body.innerHTML = bodyHtml;
  window.eval(js);

  return { window, document: window.document, clip, warns, errors, unhandled, openedUrls, confirmCalls: () => confirmCalls, dom, dims };
}

// 等终端完成握手（WELCOME 处理完）：shell prompt 出现即 OUTPUT 流通
async function waitReady(document) {
  await waitFor(() => document.querySelector('.xterm-rows')?.textContent?.length > 0, '终端首输出');
}

// 向 shell 发送文本。xterm 6 输入链（bundle _inputEvent 实测核实）：
// 可打印字符走 textarea 的 InputEvent(inputType='insertText', data=...) →
// coreService.triggerDataEvent → term.onData → WS INPUT；Enter 走 keydown(keyCode=13)。
// jsdom 不实现 keydown 的默认文本插入，故字符必须经 InputEvent 注入。
// 整串一次注入（非逐字符）——实测逐字符派发反斜杠会丢（dead-key 处理窗口竞态），
// 且整串形态更接近真实浏览器文本插入语义
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

// 读终端某行文本（DOM 渲染器 .xterm-rows 按行 div）
function rowText(document, i) {
  const rows = document.querySelectorAll('.xterm-rows > div');
  return rows[i]?.textContent ?? '';
}
function allText(document) {
  return document.querySelector('.xterm-rows')?.textContent ?? '';
}

// 鼠标事件派发（clientX/Y 经布局桩换算 cell）
function mouse(window, type, x, y, target) {
  // detail:1 = 单击语义（handleMouseDown 按 detail 分派单/双/三击，jsdom 默认 0 不落任何分支）；
  // 'mousemove-drag' 是调用别名——派发真实类型 mousemove 但 buttons=1（左键拖拽）；
  // 纯 hover 用 'mousemove'（buttons=0——buttons&1 时 link hover 链不触发，OSC8 实测）
  const realType = type === 'mousemove-drag' ? 'mousemove' : type;
  const el = target ?? window.document;
  el.dispatchEvent(new window.MouseEvent(realType, {
    bubbles: true, cancelable: true, clientX: x, clientY: y, button: 0, detail: 1,
    buttons: type === 'mousedown' || type === 'mousemove-drag' ? 1 : 0,
  }));
}

// 统一清理：先杀服务端让 WS 断开落定，再关 jsdom——顺序颠倒会让 bundle 内
// 回调（onclose→showStatus 等）访问已销毁 document 抛 TypeError（实测）
async function cleanup(ctx, srv) {
  try { srv.kill(); } catch { /* 已退出忽略 */ }
  await sleep(150);
  try { ctx.dom.window.close(); } catch { /* 忽略 */ }
}

// ═══════════════════ 场景 ═══════════════════

// T3 标题同步：形态 A OSC 2 改标题；形态 B 恒 [ro] 前缀
// 注意：必须用 bash --norc——/etc/bashrc 的 PROMPT_COMMAND 每个 prompt 发 OSC 0 重置
// 标题（实测覆盖测试值），--norc 隔离该环境干扰；被测对象是 xterm onTitleChange 链
async function t3() {
  console.log('T3 标题同步（CORE-03）');
  // 形态 A：rw
  let srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash', '--norc']);
  let ctx = await loadTerminal(srv);
  await waitReady(ctx.document);
  typeText(ctx.window, "printf '\\e]2;custom-title\\a'\n");
  await waitFor(() => ctx.document.title === 'custom-title', '标题同步为 custom-title');
  check('T3-A', 'rw 形态标题同步', true);
  // 多次变化
  typeText(ctx.window, "printf '\\e]2;second\\a'\n");
  await waitFor(() => ctx.document.title === 'second', '标题二次变化');
  check('T3-A2', '标题随程序多次变化', true);
  await cleanup(ctx, srv);

  // 形态 B：ro。注意 ro 下 disableStdin=true（main.ts:391）——键盘输入被禁，
  // printf 类主动 OSC 不可达（原 UAT "形态 B 重复" 步骤设计缺陷，真实环境同样不可输入）。
  // 改用普通 bash（无 --norc）：/etc/bashrc PROMPT_COMMAND 每个 prompt 自发 OSC 0，
  // 正好驱动 onTitleChange 链；断言前缀恒最前且不双。
  srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--', 'bash']);
  ctx = await loadTerminal(srv);
  await waitReady(ctx.document);
  // WELCOME 处理与首输出无严格先后（jsdom 事件循环调度）——轮询等 [ro] 前缀出现
  await waitFor(() => ctx.document.title.startsWith('[ro] '), 'ro WELCOME 置前缀');
  check('T3-B0', 'ro 初始标题带 [ro] 前缀', true, `title=${ctx.document.title}`);
  // PROMPT_COMMAND 发 OSC 0 后：标题变为 [ro] user@host 形态（前缀最前）
  await waitFor(() => ctx.document.title.startsWith('[ro] ') && ctx.document.title !== '[ro] wesh', 'ro OSC 驱动标题变化');
  check('T3-B', 'ro 形态程序自发 OSC 标题同步（前缀最前）', true, `title=${ctx.document.title.slice(0, 40)}`);
  await sleep(600); // 覆盖多个 prompt 周期
  check('T3-B2', 'ro 多次变化前缀不双不丢', !ctx.document.title.startsWith('[ro] [ro]') && ctx.document.title.startsWith('[ro] '), `title=${ctx.document.title.slice(0, 40)}`);
  await cleanup(ctx, srv);
}

// T9 beforeunload：会话中拦截 / Session ended 后放行 / 开关关闭不拦截
async function t9() {
  console.log('T9 离开确认 beforeunload（FE-06 / D-18）');
  // 会话中：dispatch cancelable beforeunload → defaultPrevented 应为 true
  let srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  let ctx = await loadTerminal(srv);
  await waitReady(ctx.document);
  // 注册点在 WELCOME 处理完成（welcomeDone 门），与首输出无严格先后——轮询派发直到拦截
  await waitFor(() => {
    const e = new ctx.window.Event('beforeunload', { cancelable: true });
    ctx.window.dispatchEvent(e);
    return e.defaultPrevented === true;
  }, 'beforeunload 注册（WELCOME 完成）');
  check('T9-1', '会话中关页拦截（preventDefault）', true);
  // 会话终结：exit → onclose 1000 → 移除监听 → 再放行
  typeText(ctx.window, 'exit\n');
  await waitFor(() => ctx.document.querySelector('#status')?.hidden === false, 'Session ended 面板');
  const ev = new ctx.window.Event('beforeunload', { cancelable: true });
  ctx.window.dispatchEvent(ev);
  check('T9-2', '会话终结后不再拦截', ev.defaultPrevented === false);
  await cleanup(ctx, srv);

  // 开关关闭：?confirmBeforeUnload=false（注册点条件跳过——事件永不拦截。
  // 注意须等 WELCOME 完成再断言，否则测的是"尚未注册"假阴性）
  srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  ctx = await loadTerminal(srv, { query: '?confirmBeforeUnload=false' });
  await waitReady(ctx.document);
  await sleep(300); // 等 WELCOME 处理窗口（对照 T9-1 实测 300ms 内必注册）
  const ev3 = new ctx.window.Event('beforeunload', { cancelable: true });
  ctx.window.dispatchEvent(ev3);
  check('T9-3', 'confirmBeforeUnload=false 不拦截', ev3.defaultPrevented === false);
  await cleanup(ctx, srv);
}

// T8 resize 浮层：onResize 驱动 COLSxROWS 显示与 600ms 后淡出；开关关闭不显示
async function t8() {
  console.log('T8 resize 浮层（FE-06 / D-17）');
  const srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  const ctx = await loadTerminal(srv);
  await waitReady(ctx.document);
  const overlay = ctx.document.getElementById('resize-overlay');
  check('T8-0', '初始浮层隐藏', overlay.hidden === true);
  // 改 dims 桩（#terminal computed width/height 唯一读取点 = fit proposeDimensions）
  // → window resize 事件 → 100ms debounce → fit() → onResize → 浮层
  ctx.dims.w = 900;
  ctx.window.dispatchEvent(new ctx.window.Event('resize'));
  await waitFor(() => overlay.hidden === false && /^\d+x\d+$/.test(overlay.textContent), '浮层出现 COLSxROWS');
  check('T8-1', 'resize 期间浮层显示 COLSxROWS', true, overlay.textContent);
  check('T8-2', '浮层初始不透明', overlay.style.opacity === '1');
  await waitFor(() => overlay.style.opacity === '0', '600ms 后淡出', 2000);
  check('T8-3', '静止约 600ms 后淡出', true);
  await cleanup(ctx, srv);

  // 开关关闭：?resizeOverlay=false
  const srv2 = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  const ctx2 = await loadTerminal(srv2, { query: '?resizeOverlay=false' });
  await waitReady(ctx2.document);
  ctx2.dims.w = 900;
  ctx2.window.dispatchEvent(new ctx2.window.Event('resize'));
  await sleep(400); // 等过 100ms debounce + 事件链
  const ov2 = ctx2.document.getElementById('resize-overlay');
  check('T8-4', 'resizeOverlay=false 不显示', ov2.hidden === true);
  await cleanup(ctx2, srv2);
}

// T10 偏好下发与 query 覆盖
async function t10() {
  console.log('T10 偏好下发与 query 覆盖（FE-07 / D-16 / D-19）');
  // --client-option fontSize=18 + theme background
  let srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable',
    '--client-option', 'fontSize=18', '--client-option', 'theme={"background":"#101020"}', '--', 'bash']);
  let ctx = await loadTerminal(srv);
  await waitReady(ctx.document);
  // fontSize 的 DOM 落点实测为 .xterm-width-cache-measure-container（xterm 6 行内
  // fontSize 唯一内联写点）；theme background 落在 .xterm-viewport 背景
  await waitFor(() => /18px/.test(ctx.document.querySelector('.xterm-width-cache-measure-container')?.style.fontSize ?? ''), 'fontSize=18 应用');
  check('T10-1', '服务端 fontSize=18 生效', true);
  await waitFor(() => {
    // theme background 的 DOM 落点实测为 .xterm-scrollable-element 行内背景
    const bg = ctx.document.querySelector('.xterm-scrollable-element')?.style.backgroundColor ?? '';
    return bg.includes('16, 16, 32');
  }, 'theme background 应用');
  check('T10-2', '服务端 theme.background #101020 生效', true);
  // ANSI 彩色保持：内置调色板 red #cc0000 未指定不丢——发红色输出。
  // xterm 6 DOM 渲染器的 ANSI 色用动态 CSS class（.xterm-fg-1）而非 inline style（实测），
  // 色值在动态样式表规则里——断言 class 附着 + 规则色值仍为 tango red
  typeText(ctx.window, "printf '\\e[31mRED\\e[0m\\n'\n");
  await waitFor(() => [...ctx.document.querySelectorAll('.xterm-rows span')].some((s) => s.classList.contains('xterm-fg-1') && s.textContent.includes('RED')), 'ANSI red class 附着');
  const fg1Rule = [...ctx.document.styleSheets].flatMap((sh) => [...sh.cssRules]).find((r) => r.selectorText?.endsWith(' .xterm-fg-1'));
  check('T10-3', 'theme 合并未指定键不丢（ANSI red 保持内置）', fg1Rule?.style.color === '#cc0000', fg1Rule?.style.color ?? 'rule-missing');
  await cleanup(ctx, srv);

  // query 覆盖：同 flag 启动 + ?fontSize=11 → 11 胜
  srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--client-option', 'fontSize=18', '--', 'bash']);
  ctx = await loadTerminal(srv, { query: '?fontSize=11' });
  await waitReady(ctx.document);
  await waitFor(() => /11px/.test(ctx.document.querySelector('.xterm-width-cache-measure-container')?.style.fontSize ?? ''), 'query fontSize=11 覆盖');
  check('T10-4', 'URL query 覆盖 --client-option（优先级）', true);
  await cleanup(ctx, srv);

  // 非法 query：?fontFamily=Menlo（裸词非法 JSON）→ 静默忽略 + warn + 终端可用
  srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  ctx = await loadTerminal(srv, { query: '?fontFamily=Menlo' });
  await waitReady(ctx.document);
  await waitFor(() => ctx.warns.some((w) => w.includes('ignoring invalid query pref') && w.includes('fontFamily')), '非法 query warn');
  check('T10-5', '非法 query 静默忽略 + console.warn', true);
  typeText(ctx.window, 'echo STILL-OK\n');
  await waitFor(() => allText(ctx.document).includes('STILL-OK'), '终端仍可用');
  check('T10-6', '非法 query 后终端正常可用', true);
  await cleanup(ctx, srv);
}

// T6 粘贴：rw 形态 Ctrl+Shift+V 内容到达 shell；ro 形态不读剪贴板
async function t6() {
  console.log('T6 粘贴（FE-05 / D-10）');
  // rw：mock 剪贴板预置内容，按 Ctrl+Shift+V → 回显到终端
  let srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  let ctx = await loadTerminal(srv, { clipboard: 'mock' });
  await waitReady(ctx.document);
  ctx.clip.writes.push('PASTE-MARK-123');
  const readsBefore = ctx.clip.reads;
  ctx.window.dispatchEvent(new ctx.window.KeyboardEvent('keydown', { key: 'v', ctrlKey: true, shiftKey: true, bubbles: true, cancelable: true }));
  await waitFor(() => allText(ctx.document).includes('PASTE-MARK-123'), '粘贴内容回显');
  check('T6-1', 'rw 粘贴内容到达 shell', true);
  check('T6-2', 'rw 路径读取剪贴板一次', ctx.clip.reads === readsBefore + 1);
  await cleanup(ctx, srv);

  // ro：isRO 门——readText 不应被调用（无权限弹窗的代码面等价断言）
  srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--', 'bash']);
  ctx = await loadTerminal(srv, { clipboard: 'mock' });
  await waitReady(ctx.document);
  await waitFor(() => ctx.document.title.startsWith('[ro] '), 'ro WELCOME 到达');
  ctx.clip.writes.push('SHOULD-NOT-READ');
  const roReadsBefore = ctx.clip.reads;
  ctx.window.dispatchEvent(new ctx.window.KeyboardEvent('keydown', { key: 'v', ctrlKey: true, shiftKey: true, bubbles: true, cancelable: true }));
  await sleep(300);
  check('T6-3', 'ro 形态不读剪贴板（无权限弹窗）', ctx.clip.reads === roReadsBefore);
  check('T6-4', 'ro 形态无内容到达终端', !allText(ctx.document).includes('SHOULD-NOT-READ'));
  await cleanup(ctx, srv);
}

// T5 选中即复制：拖动选中 → 150ms 防抖 → 剪贴板一次写入
async function t5() {
  console.log('T5 选中即复制（FE-05）');
  const srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  const ctx = await loadTerminal(srv, { clipboard: 'mock' });
  await waitReady(ctx.document);
  typeText(ctx.window, 'echo SELECT-ME-NOW\n');
  await waitFor(() => allText(ctx.document).includes('SELECT-ME-NOW'), '目标文本上屏');
  // 找到目标行索引，对其做拖动选中（行 0 起扫）
  const rows = [...ctx.document.querySelectorAll('.xterm-rows > div')];
  const rowIdx = rows.findIndex((r) => r.textContent.includes('SELECT-ME-NOW'));
  const y = rowIdx * 17 + 8; // 行内垂直中点
  const screen = ctx.document.querySelector('.xterm-screen');
  // mousedown → 数次 mousemove（拖动过程）→ mouseup
  mouse(ctx.window, 'mousedown', 0, 4, screen);
  for (let x = 10; x <= 130; x += 30) mouse(ctx.window, 'mousemove-drag', x, y, screen);
  mouse(ctx.window, 'mouseup', 140, y, screen);
  await sleep(400); // 150ms 防抖 + 余量
  const joined = ctx.clip.writes.join('|');
  check('T5-1', '选中后剪贴板写入发生', ctx.clip.writes.length >= 1, `writes=${ctx.clip.writes.length}`);
  check('T5-2', '拖动过程合并写入（防抖，≤2 次）', ctx.clip.writes.length <= 2, `writes=${ctx.clip.writes.length}`);
  check('T5-3', '剪贴板含所选文本片段', /SELECT|ME-NOW/.test(joined), joined.slice(0, 60));
  await cleanup(ctx, srv);
}

// T7 明文降级形态：navigator.clipboard 缺席 → 复制/粘贴静默不生效，终端其余正常
async function t7() {
  console.log('T7 明文降级（D-11，clipboardOK=false 分支）');
  // jsdom navigator 无 clipboard → clipboardOK 天然 false，等价明文非 localhost 形态
  const srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  const ctx = await loadTerminal(srv, { clipboard: 'absent' });
  await waitReady(ctx.document);
  check('T7-0', 'clipboardOK=false（navigator.clipboard 缺席）', typeof ctx.window.navigator.clipboard === 'undefined');
  // 拖动选中：不应抛错、无任何写入路径
  const screen = ctx.document.querySelector('.xterm-screen');
  mouse(ctx.window, 'mousedown', 0, 4, screen);
  mouse(ctx.window, 'mousemove-drag', 60, 12, screen);
  mouse(ctx.window, 'mouseup', 120, 12, screen);
  await sleep(300);
  // Ctrl+Shift+V：keydown handler 首行 return
  ctx.window.dispatchEvent(new ctx.window.KeyboardEvent('keydown', { key: 'v', ctrlKey: true, shiftKey: true, bubbles: true, cancelable: true }));
  await sleep(200);
  check('T7-1', '选中/粘贴静默无异常', ctx.unhandled.length === 0, `unhandled=${ctx.unhandled.length}`);
  // 终端其余功能正常
  typeText(ctx.window, 'echo DEGRADE-OK\n');
  await waitFor(() => allText(ctx.document).includes('DEGRADE-OK'), '终端回显正常');
  check('T7-2', '降级形态终端显示/输入正常', true);
  await cleanup(ctx, srv);
}

// T11 OSC52 写入：--osc52 + printf OSC52 写序列 → 剪贴板 hello；读查询无 unhandled rejection
async function t11() {
  console.log('T11 OSC52 写入（D-12）');
  const srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--osc52', '--', 'bash']);
  const ctx = await loadTerminal(srv, { clipboard: 'mock' });
  await waitReady(ctx.document);
  typeText(ctx.window, "printf '\\e]52;c;aGVsbG8=\\a'\n");
  await waitFor(() => ctx.clip.writes.includes('hello'), 'OSC52 写入 hello');
  check('T11-1', 'OSC52 写序列落系统剪贴板', true);
  typeText(ctx.window, "printf '\\e]52;c;?\\a'\n");
  await sleep(400);
  check('T11-2', 'OSC52 读查询无 unhandled rejection', ctx.unhandled.length === 0, `unhandled=${ctx.unhandled.length}`);
  await cleanup(ctx, srv);
}

// T2 IME 组合输入：合成 CompositionEvent 链 → 上屏文本经 onData 到 shell 回显
async function t2() {
  console.log('T2 IME 组合输入（FE-02 逻辑面：composition 事件链）');
  const srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  const ctx = await loadTerminal(srv);
  await waitReady(ctx.document);
  const ta = ctx.document.querySelector('.xterm-helper-textarea');
  ta.focus();
  // 合成拼音组合过程：compositionstart → update(中间态) → end(上屏) → input
  const CompEv = ctx.window.CompositionEvent ?? ctx.window.Event;
  ta.dispatchEvent(new CompEv('compositionstart', { bubbles: true, data: '' }));
  ta.value = 'zhong';
  ta.dispatchEvent(new CompEv('compositionupdate', { bubbles: true, data: 'zhong' }));
  ta.value = '中文';
  ta.dispatchEvent(new CompEv('compositionupdate', { bubbles: true, data: '中文' }));
  ta.dispatchEvent(new CompEv('compositionend', { bubbles: true, data: '中文' }));
  ta.dispatchEvent(new ctx.window.Event('input', { bubbles: true }));
  // 上屏文本应到达 shell 并回显（bash readline 回显 IME 上屏字符）
  await waitFor(() => allText(ctx.document).includes('中文'), 'IME 上屏文本回显');
  check('T2-1', 'composition 链上屏不丢字', true);
  // 回车执行：把 中文 作为命令回车——shell 报 command not found 即证明字节完整到达
  ta.value = '';
  ta.dispatchEvent(new ctx.window.Event('input', { bubbles: true }));
  typeText(ctx.window, '\n');
  await waitFor(() => /中文.*not found|中文.*command/i.test(allText(ctx.document)), 'shell 收到完整 IME 字节');
  check('T2-2', 'IME 字节完整到达 shell（command not found 回证）', true);
  await cleanup(ctx, srv);
}

// T4 超链接：自动识别 URL hover tooltip + 点击 window.open；OSC8 无 confirm
async function t4() {
  console.log('T4 超链接（FE-04 逻辑面）');
  const srv = await startWesh(['--bind', '127.0.0.1', '--port', '0', '--writable', '--', 'bash']);
  const ctx = await loadTerminal(srv);
  await waitReady(ctx.document);
  // 让 URL 落在已知行：清屏后第一行输出
  typeText(ctx.window, "clear; echo 'see https://example.com now'\n");
  await waitFor(() => allText(ctx.document).includes('https://example.com'), 'URL 上屏');
  const rows = [...ctx.document.querySelectorAll('.xterm-rows > div')];
  const rowIdx = rows.findIndex((r) => r.textContent.includes('https://example.com'));
  const colIdx = rows[rowIdx].textContent.indexOf('https://example.com');
  const x = (colIdx + 3) * 9, y = rowIdx * 17 + 8; // URL 中段字符
  // hover：web-links addon 经 mousemove 探测
  mouse(ctx.window, 'mousemove', x, y, ctx.document.querySelector('.xterm-screen'));
  const tooltip = await waitFor(() => {
    const t = ctx.document.querySelector('.xterm-hover');
    return t && t.style.display !== 'none' && t.textContent.includes('https://example.com') ? t : null;
  }, 'hover tooltip 出现');
  check('T4-1', 'hover 显示完整真实地址 tooltip', tooltip.textContent.includes('https://example.com'), tooltip.textContent.slice(0, 50));
  // 点击激活：mousedown+mouseup 同位置 → click → window.open
  const before = ctx.openedUrls.length;
  mouse(ctx.window, 'mousedown', x, y, ctx.document.querySelector('.xterm-screen'));
  mouse(ctx.window, 'mouseup', x, y, ctx.document.querySelector('.xterm-screen'));
  await sleep(200);
  check('T4-2', '单击触发 window.open（新标签页语义）', ctx.openedUrls.length > before, `opens=${ctx.openedUrls.length}`);

  // OSC 8：显示文本与目标不同；hover tooltip 显示真实目标；点击无 confirm
  typeText(ctx.window, "printf '\\e]8;;https://osc8-target.example\\aSHOW-TEXT\\e]8;;\\a'\n");
  // 等 printf 真正执行完：命令回显折行里也有字面量 SHOW-TEXT，allText 匹配会抢跑——
  // 输出行的特征是"以 SHOW-TEXT 开头"（printf 输出于行首；回显行以 prompt 或命令片段开头）
  await waitFor(() => [...ctx.document.querySelectorAll('.xterm-rows > div')].some((r) => r.textContent.startsWith('SHOW-TEXT')), 'OSC8 显示文本上屏');
  const rows2 = [...ctx.document.querySelectorAll('.xterm-rows > div')];
  const r2 = rows2.findLastIndex((r) => r.textContent.includes('SHOW-TEXT'));
  const c2 = rows2[r2].textContent.indexOf('SHOW-TEXT');
  const x2 = (c2 + 2) * 9, y2 = r2 * 17 + 8;
  // hover 前先把鼠标移到空白区——清除前次 web-links hover 的 link 态
  // （不先 leave 直接跳到 OSC8 位置，实测 hover 回调不触发）
  mouse(ctx.window, 'mousemove', 700, 400, ctx.document.querySelector('.xterm-screen'));
  await sleep(150);
  mouse(ctx.window, 'mousemove', x2, y2, ctx.document.querySelector('.xterm-screen'));
  const tip2 = await waitFor(() => {
    const t = ctx.document.querySelector('.xterm-hover');
    return t && t.style.display !== 'none' && t.textContent.includes('osc8-target.example') ? t : null;
  }, 'OSC8 hover tooltip 真实目标');
  check('T4-3', 'OSC8 hover 显示真实目标（与显示文本可辨别）', tip2.textContent.includes('https://osc8-target.example'), tip2.textContent.slice(0, 60));
  const confirmBefore = ctx.confirmCalls();
  mouse(ctx.window, 'mousedown', x2, y2, ctx.document.querySelector('.xterm-screen'));
  mouse(ctx.window, 'mouseup', x2, y2, ctx.document.querySelector('.xterm-screen'));
  await sleep(200);
  check('T4-4', 'OSC8 点击无 confirm 原生框', ctx.confirmCalls() === confirmBefore);
  await cleanup(ctx, srv);
}

// ═══════════════════ 主流程 ═══════════════════
const t0 = Date.now();
console.log(`wesh: ${WESH}\ndist: ${DIST}\n`);
const suites = { t2, t3, t4, t5, t6, t7, t8, t9, t10, t11 };
const only = process.argv[3];
for (const [name, fn] of Object.entries(suites)) {
  if (only && name !== only) continue;
  try {
    await fn();
  } catch (e) {
    check(name.toUpperCase(), '场景异常中断', false, String(e).slice(0, 120));
  }
}
const pass = results.filter((r) => r.ok).length;
console.log(`\n${pass}/${results.length} PASS  (${((Date.now() - t0) / 1000).toFixed(1)}s)`);
process.exit(pass === results.length ? 0 : 1);
