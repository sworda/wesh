import '@xterm/xterm/css/xterm.css'; // xterm 必需样式，singlefile 内联
import { Terminal, type ITerminalOptions } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { ClipboardAddon, type IClipboardProvider } from '@xterm/addon-clipboard'; // 仅 WELCOME prefs osc52===true 时条件加载（D-12）
import { sanitizeTitle } from './lib/title';
import { parseQueryPrefs, splitPrefs, mergeTheme } from './lib/prefs';

// 帧常量与 internal/proto/proto.go 手工对齐（D-16，两侧注释互相指路）：
// '0' INPUT / '1' RESIZE / '0' OUTPUT / 'H' Hello / 'W' Welcome / 'E' Error；
// SUBPROTOCOL 同时是 WS 子协议 token 与 Hello.version 期望值（D-03，同源复用防双写漂移）。
// Hello 载荷 {version, cols, rows, ticket?}——ticket 为 Phase 3 认证核销一次性票（可选，
// 无认证模式省略该键，proto.go HelloPayload.Ticket omitempty 同形）；
// Welcome 载荷 {mode, prefs?}——prefs 为可选偏好下发字段（D-13 omitempty，proto.go WelcomePayload.Prefs 同形）；
// Error code 含 auth_failed（ticket 核销失败统一口径 D-10，proto.go ErrAuthFailed，前端据此静默重试一次）。
const OUTPUT = 0x30,
  INPUT = 0x30,
  RESIZE = 0x31,
  HELLO = 0x48,
  WELCOME = 0x57,
  ERROR = 0x45,
  SUBPROTOCOL = 'wesh.v1';

// wesh tango 调色板常量化（RESEARCH §Pitfall 3 修正前提——运行期与构造期
// {...defaultTheme, ...incoming} 合并需要构造时对象可引用；调色板单一事实源：
// 构造初值、query theme 特判合并与 WELCOME prefs theme 合并三处同源）
const defaultTheme = {
  foreground: '#ffffff',
  background: '#000000',
  cursor: '#ffffff',
  cursorAccent: '#000000',
  selectionBackground: 'rgba(255,255,255,0.3)',
  black: '#2e3436',
  brightBlack: '#555753',
  red: '#cc0000',
  brightRed: '#ef2929',
  green: '#4e9a06',
  brightGreen: '#8ae234',
  yellow: '#c4a000',
  brightYellow: '#fce94f',
  blue: '#3465a4',
  brightBlue: '#729fcf',
  magenta: '#75507b',
  brightMagenta: '#ad7fa8',
  cyan: '#06989a',
  brightCyan: '#34e2e2',
  white: '#d3d7cf',
  brightWhite: '#eeeeec',
} as const;

// FE-07 query 通道（UI-SPEC §Prefs Contract 装配顺序步骤 1）：Terminal 构造前解析
// location.search——白名单键经 JSON.parse 成功者记入 queryKeys（WELCOME prefs 应用时跳过，
// 优先级 URL query > --client-option > 内置默认，D-16）并作构造选项初值；
// 非法 JSON/白名单外键静默忽略 + console.warn（用户侧输入不该让终端不可用，D-16）
const query = parseQueryPrefs(location.search);
// WELCOME prefs 应用段经 queryKeys 跳过 query 已设键——export 防 noUnusedLocals 在接线前误报
export const queryKeys = query.keys;
for (const k of query.invalid) {
  console.warn('ignoring invalid query pref:', k);
}
const queryParts = splitPrefs(query.prefs);
// query theme 构造特判（RESEARCH §Pitfall 3 修正覆盖 query 通道——部分 theme 丢调色板问题
// 不止 WELCOME 路径，两通道行为须一致）：部分 theme 与 defaultTheme 合并后作构造 theme，
// 未指定键保留 tango 调色板；非对象 theme 值中和为默认调色板（D-16 容错终端不挂）
if ('theme' in queryParts.xterm) {
  const merged = mergeTheme(defaultTheme, queryParts.xterm.theme);
  if (merged !== null) {
    queryParts.xterm.theme = merged;
  } else {
    console.warn('ignoring non-object query theme, falling back to default palette');
    queryParts.xterm.theme = defaultTheme;
  }
}

// Terminal Options 按 UI-SPEC §Terminal Options Contract 逐项显式钉死，不依赖库默认值
const term = new Terminal({
  fontSize: 14,
  fontFamily:
    'Menlo, Monaco, "Cascadia Mono", Consolas, "DejaVu Sans Mono", "Liberation Mono", "Courier New", monospace',
  lineHeight: 1.0,
  letterSpacing: 0,
  fontWeight: 400,
  fontWeightBold: 700,
  cursorStyle: 'block',
  cursorBlink: true, // 有意覆盖 xterm 默认 false——闪烁光标是输入点的关键可见性提示
  scrollback: 10000,
  allowTransparency: false,
  allowProposedApi: true, // Unicode11Addon 依赖的 unicode API 在 xterm 6 仍为 EXPERIMENTAL——
  // 不设 true 则 loadAddon 即抛错，模块顶层无 try 整个前端中止（04-UAT jsdom 自动化抓到的 P0）
  theme: defaultTheme,
  // query 的 xterm 键作构造初值（内置默认 ← query 覆盖先行，UI-SPEC 装配顺序步骤 1）；
  // 白名单已保证键合法性，经一次收窄 cast（lib/prefs.ts 与 Go 侧 ValidClientOptionKey 同源）
  ...queryParts.xterm as Partial<ITerminalOptions>,
});

const fit = new FitAddon();
term.loadAddon(fit);
term.open(document.getElementById('terminal')!); // 同步于 WS 连接前打开——空黑终端即加载态，无 spinner

// FE-01：首选 WebGL 渲染器，加载失败停留 DOM 渲染器（xterm.js 6 已无 canvas，回落目标唯一）
try {
  const webglAddon = new WebglAddon();
  // GPU 上下文丢失（驱动重置/睡眠唤醒）自动回落 DOM，不黑屏（UI-SPEC §Renderer Contract）
  webglAddon.onContextLoss(() => webglAddon.dispose());
  term.loadAddon(webglAddon);
} catch (e) {
  console.warn('webgl addon load failed, stay on DOM renderer', e);
}

// FE-02：Unicode 11 宽度测量（CJK/emoji 正确占两格）。硬顺序（RESEARCH §Pitfall 2）——
// loadAddon 仅注册 provider，必须紧随 activeVersion='11' 激活；仅加载不激活等于没装
// （宽度仍走 Unicode 6），顺序颠倒则 setter 抛 unknown Unicode version "11"
term.loadAddon(new Unicode11Addon());
term.unicode.activeVersion = '11';
// FE-04 通道①：文本 URL 链接化。第一参 undefined = 库默认 handler（window.open →
// opener=null → location.href，等价 target=_blank rel=noopener，单击激活无修饰键）——
// 不传自定义 handler、不自定义正则（D-05/D-07 锁定库默认：自维护正则与自写打开包装都是
// 重复造轮子且易丢 opener=null 防 reverse tabnapping）；0.12.0 默认正则实为仅 http(s)
term.loadAddon(new WebLinksAddon(undefined, { hover: showLinkTooltip, leave: hideLinkTooltip }));

// FE-04 通道②：OSC 8 显式超链接。必须显式设 linkHandler——不设则核心回退 confirm() 原生
// 警告框（xterm.d.ts 明示，D-06）；activate 走与库默认 handleLink 同形态（window.open +
// opener=null）。allowNonHttpProtocols 不写保持默认 false——javascript:/file: 等 OSC8
// 被结构性忽略（钓鱼面纵深防御）
term.options.linkHandler = {
  activate: (_event, uri) => {
    const w = window.open();
    if (w) {
      w.opener = null;
      w.location.href = uri;
    }
  },
  hover: (event, text) => showLinkTooltip(event, text),
  leave: () => hideLinkTooltip(),
};

// hover 真实 URL tooltip（D-06 钓鱼辨别：OSC8 显示文本可与真实 uri 不同，双通道统一展示）。
// div 带 xterm-hover class 创建于 term.element 内——核心 hover 路径对该 class 提前 return，
// 防 tooltip 自身触发链接 enter/leave 抖动（RESEARCH §Pattern 3 核实注）；
// hover 即显 leave 即隐，无延迟定时器（UI-SPEC §Link Tooltip Spec）
const linkTooltip = document.createElement('div');
linkTooltip.classList.add('xterm-hover');
term.element!.appendChild(linkTooltip);

function showLinkTooltip(event: MouseEvent, text: string): void {
  linkTooltip.textContent = text; // 完整 URL 原文，无前缀不截断（CSS break-all 折行展示全文）
  linkTooltip.style.display = 'block';
  // 指针右下 +8px 偏移；视口右/下边缘不足时向左/上翻转防溢出
  let left = event.clientX + 8;
  let top = event.clientY + 8;
  if (left + linkTooltip.offsetWidth > window.innerWidth) {
    left = event.clientX - 8 - linkTooltip.offsetWidth;
  }
  if (top + linkTooltip.offsetHeight > window.innerHeight) {
    top = event.clientY - 8 - linkTooltip.offsetHeight;
  }
  linkTooltip.style.left = `${Math.max(0, left)}px`;
  linkTooltip.style.top = `${Math.max(0, top)}px`;
}

function hideLinkTooltip(): void {
  linkTooltip.style.display = 'none';
}

let opened = false; // 是否曾成功 onopen——三态文案分派依据（UI-SPEC §Copywriting）
let helloSent = false; // Hello 首帧发出后才置位——此前 sendResize 门吞掉全部数据帧（防 RESIZE 抢跑被 1002 直关）
let lastError: { code: string; message: string } | null = null; // 最近一帧 Error{code,message}，onclose 展示用（D-07）
// 连接句柄提升为模块级：connect() 内赋值，onData/sendResize 等常驻接线引用当前连接；
// 首连前 fetch 窗口内为 null（此间用户敲击被 null 闸静默吞掉，不抛 TypeError）
let ws: WebSocket | null = null;
let retriedAuth = false; // auth_failed 静默重试仅一次的门闩（D-10；无限重试会把 60s TTL 正常过期放大成重试风暴）
// ro 判定模块级化——标题写口/04-03 粘贴门/04-05 osc52 门三处共用（RESEARCH §Pattern 6 核实注）
let isRO = false;
// 最近一次远程标题的 sanitize 后形态——[ro] 前缀组合与 auth_failed 重试重前缀防御的单一事实源
let remoteTitle = 'wesh';
// FE-06 三开关量（04-05 经 query/prefs 翻转接线，本 plan 先取默认值）：
// welcomeDone —— WELCOME 处理完成置位（浮层与离开确认的会话建立门）；
// resizeOverlayOn —— resize 浮层默认开（D-17）；confirmBeforeUnloadOn —— 离开页面前确认默认开（D-18）
let welcomeDone = false;
let resizeOverlayOn = true;
let confirmBeforeUnloadOn = true;

// query 的 behavior 键即 startup 应用（typeof boolean 校验——服务端对值只验 JSON 不验类型，
// 前端防御性应用；非布尔忽略 + console.warn）；位置在 WELCOME 分支 welcomeDone 置位与
// beforeunload 条件注册点之前——开关值在注册点已是最终态
for (const [k, v] of Object.entries(queryParts.behavior)) {
  if (typeof v !== 'boolean') {
    console.warn(`ignoring non-boolean behavior pref: ${k}`);
    continue;
  }
  if (k === 'resizeOverlay') {
    resizeOverlayOn = v;
  } else if (k === 'confirmBeforeUnload') {
    confirmBeforeUnloadOn = v;
  }
}

// FE-06 离开页面前确认（D-18）：仅 preventDefault() 触发浏览器标准确认框——自定义文案
// 被现代浏览器一律忽略故不写（MDN beforeunload_event，RESEARCH §Pattern 5 核实注）；
// 零交互直接关页不弹框为浏览器预期语义（sticky activation，RESEARCH §Open Questions 3 已裁决接受）
const onBeforeUnload = (e: BeforeUnloadEvent): void => {
  e.preventDefault();
};

// CORE-03 标题单一写口（D-02 前缀恒最前 + D-04 无品牌后缀）：
// document.title 只允许经本函数写入，sanitize 已在 onTitleChange 入口完成不可旁路
function setTitle(): void {
  document.title = (isRO ? '[ro] ' : '') + remoteTitle;
}

function concat(...parts: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) {
    out.set(p, off);
    off += p.length;
  }
  return out;
}

// C→S：键盘输入（CJK/IME 由 xterm 内部 composition 处理，onData 交付最终字符串）；
// 仅 OPEN 时发送——面板显示期间输入静默丢弃（UI-SPEC §Interaction Contract）
const enc = new TextEncoder();
term.onData((s) => {
  if (ws !== null && ws.readyState === WebSocket.OPEN) {
    ws.send(concat(new Uint8Array([INPUT]), enc.encode(s)));
  }
});

// FE-03 + CORE-02：resize 回路。window resize → 100ms debounce → fit() →
// onResize → RESIZE 帧（JSON {"cols","rows"}，服务端钳制 [1,1000]）。
// 发送前防护：display:none 时 proposeDimensions 返回无效值（PITFALLS C10）。
function sendResize(cols: number, rows: number): void {
  // Hello 完成前禁发任何数据帧：term.onResize 常驻接线在首次 fit.fit() 几乎必然触发
  // sendResize——不门住则 RESIZE 抢跑首帧，服务端握手段以 1002 frame_before_hello 直关
  if (!helloSent) return;
  if (ws === null || ws.readyState !== WebSocket.OPEN) return;
  if (!Number.isInteger(cols) || cols <= 0 || !Number.isInteger(rows) || rows <= 0) return;
  ws.send(concat(new Uint8Array([RESIZE]), enc.encode(JSON.stringify({ cols, rows }))));
}
let overlayTimer: number | undefined;
term.onResize(({ cols, rows }) => {
  sendResize(cols, rows);
  // FE-06 resize 浮层：welcomeDone && resizeOverlayOn 双门——onopen 初次 fit 不触发
  //（welcomeDone 门：浮层是会话辅助不是启动尺寸指示器）；ro 模式同样显示
  //（ro 下 RESIZE 帧本就放行，P2 协议基线）
  if (!welcomeDone || !resizeOverlayOn) return;
  const overlay = document.getElementById('resize-overlay')!;
  overlay.textContent = `${cols}x${rows}`; // 服务端钳制 1000×1000 → 最长 9 字符无溢出（UI-SPEC §Resize Overlay Spec）
  overlay.hidden = false;
  overlay.style.opacity = '1';
  clearTimeout(overlayTimer);
  overlayTimer = window.setTimeout(() => {
    overlay.style.opacity = '0'; // 静止 600ms 后置 0，经 transition 200ms 淡出
  }, 600);
});
let timer: number | undefined;
window.addEventListener('resize', () => {
  clearTimeout(timer);
  timer = window.setTimeout(() => fit.fit(), 100);
});

// CORE-03：OSC 0/2 标题变化 → sanitize → 单一写口（D-01 纯前端解析，服务端 OUTPUT 零拷贝
// 不跑 OSC 状态机）。onTitleChange 实际由 OSC 0/2 触发——OSC 1 只设 icon name 不触发
//（RESEARCH §Pitfall 6 修正理解；真实世界标题程序均用 OSC 0/2，不为 OSC 1 写兼容代码）
term.onTitleChange((t) => {
  remoteTitle = sanitizeTitle(t);
  setTitle();
});

// FE-05：现代剪贴板（D-09/D-10/D-11）。clipboardOK 存在性门控——navigator.clipboard 是
// [SecureContext] 接口，明文 HTTP 非 localhost 下属性本身 undefined，不检测即调用抛 TypeError
//（RESEARCH §Pitfall 5）；缺失时选中复制与粘贴整体静默不生效，不落已废弃的旧 API（D-11）
const clipboardOK = typeof navigator.clipboard !== 'undefined';

// 选中即复制（D-09）：150ms trailing debounce——拖动选择期间合并只写最终选区
//（UI-SPEC §Clipboard Contract 防抖定稿）；空选区或与上次写入相同不写；
// 写失败（权限/焦点）.catch → console.warn 静默，不弹错不打断终端主流程（D-11）
let selTimer: number | undefined;
let lastCopied = '';
term.onSelectionChange(() => {
  if (!clipboardOK) return;
  clearTimeout(selTimer);
  selTimer = window.setTimeout(() => {
    const text = term.getSelection();
    if (text === '' || text === lastCopied) return;
    lastCopied = text;
    navigator.clipboard.writeText(text).catch((e) => console.warn('clipboard write failed', e));
  }, 150);
});

// Ctrl+Shift+V 粘贴（D-10）：clipboardOK 与 isRO 双门——ro 下 INPUT 本就被服务端丢弃，
// 读剪贴板只会换来无谓权限弹窗；preventDefault 阻断浏览器原生粘贴路径防双重粘贴；
// term.paste 保留 bracketed paste 语义走既有 onData→INPUT 链路（RESEARCH §Pattern 4 核实注：
// 产物 paste 路径检查 bracketedPasteMode 后 triggerDataEvent）；读拒绝 .catch 静默
window.addEventListener('keydown', (e) => {
  if (!clipboardOK || isRO) return;
  if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'v') {
    e.preventDefault();
    navigator.clipboard.readText().then((t) => term.paste(t)).catch((err) => console.warn('clipboard read failed', err));
  }
});

// #status 三态面板（UI-SPEC §Copywriting 逐字文案）：title/body + 提示行
// （提示行尾部为 accent 色 <a href="">Reload this page</a> 原地刷新链接）。
// 幂等：textContent 赋值先清空子节点再重建，onerror/onclose 双触发不重复渲染。
function showStatus(title: string, body: string, hintPrefix: string): void {
  document.getElementById('status-title')!.textContent = title;
  document.getElementById('status-body')!.textContent = body;
  const hint = document.getElementById('status-hint')!;
  hint.textContent = hintPrefix + ' ';
  const a = document.createElement('a');
  a.href = '';
  a.textContent = 'Reload this page';
  // 显式 reload——不依赖空 href 的隐式导航行为（部分情境下不可靠）
  a.addEventListener('click', (e) => {
    e.preventDefault();
    location.reload();
  });
  hint.appendChild(a);
  hint.appendChild(document.createTextNode('.'));
  document.getElementById('status')!.hidden = false;
}

// 认证感知连接流程（D-02 前端半侧）：fetch POST /api/attach 取一次性 ticket →
// 建 WS → Hello{version,cols,rows,ticket?}。ticket 只存本函数闭包变量与 Hello 载荷——
// 禁止写入 URL query/localStorage/console（T-03-24 泄漏面红线）。
async function connect(): Promise<void> {
  // 每次尝试重置 per-connection 状态——auth_failed 重试不携带上次连接残留
  opened = false;
  helloSent = false;
  lastError = null;

  let ticket: string | undefined;
  try {
    // credentials 默认 'same-origin'：浏览器 HTTP auth 缓存条目随同源 fetch 自动附带
    // （D-02 成立前提——先导航 GET / 弹原生 Basic 框缓存凭据；A2 假设，UAT 必验）
    const resp = await fetch('/api/attach', { method: 'POST' });
    if (resp.ok) {
      ticket = (await resp.json()).ticket;
    } else if (resp.status === 404) {
      // 无认证模式（--no-auth/loopback 裸跑）探测信号：跳过 ticket 直连 WS
      // （RESEARCH Pattern 1 决策；服务端无凭据时 /api/attach 显式注册 404）
      ticket = undefined;
    } else if (resp.status === 429) {
      showStatus(
        'Too many attempts',
        'Too many failed authentication attempts. The server is temporarily refusing new attempts.',
        'Wait a moment, then',
      );
      return;
    } else {
      // 401 及其余非 ok 状态同口径：通用认证失败，不细分（无 oracle 纪律延伸到前端文案）。
      // fetch 的 401 不弹浏览器原生登录框（Pitfall 6 平台行为）——引导重新加载页面，
      // 重新导航触发浏览器原生 Basic 弹窗（不自建登录表单，D-02 零新 UI 纪律）。
      showStatus(
        'Authentication failed',
        'The server rejected your credentials. Reloading the page re-opens the browser credential prompt.',
        'Check your credentials, then',
      );
      return;
    }
  } catch {
    showStatus(
      'Unable to connect',
      'The wesh server is unreachable. It may have exited, or another client is already attached (wesh currently allows a single client).',
      'Check the shell where wesh is running, then',
    );
    return;
  }

  // scheme 按页面协议选 ws/wss——https 页面下必须 wss（TLS 部署可连，03-04 伺服形态）
  ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws', [SUBPROTOCOL]); // D-03：wesh.v1 子协议建连
  const sock = ws; // 闭包内引用本连接的确定句柄（TS 对模块级可空 let 不做闭包收窄）
  sock.binaryType = 'arraybuffer';

  // S→C：按帧类型 switch 分派（与 server 握手段/数据面对称，D-01）
  sock.onmessage = (ev) => {
    const buf = new Uint8Array(ev.data as ArrayBuffer);
    switch (buf[0]) {
      case OUTPUT: // 二进制帧直写（Uint8Array 二进制安全）
        term.write(buf.subarray(1));
        break;
      case WELCOME: {
        // D-14：ro 时键盘层面即不产生 onData（UX 层，真边界在服务端丢 INPUT）
        // 畸形 JSON 负载丢弃该帧——事件处理器抛异常只丢本帧，但 WELCOME 丢失会让 ro 门失效
        try {
          const w = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
          if (w.mode === 'ro') {
            isRO = true;
            term.options.disableStdin = true;
            // 经单一写口补 [ro] 前缀——remoteTitle 不含前缀，auth_failed 重试再收
            // WELCOME 不产生 '[ro] [ro] …' 双重前缀（D-02 前缀恒最前的回归意图）
            setTitle();
          }
          // FE-07 prefs 应用段（UI-SPEC §Prefs Contract 步骤 2-4 逐字顺序：
          // xterm 键 → fit.fit() → behavior 键 → osc52 条件加载）；
          // prefs 缺省（非对象）则整段跳过——nil 兼容（omitempty 缺席即无下发）
          const prefs = w.prefs && typeof w.prefs === 'object' ? (w.prefs as Record<string, unknown>) : null;
          if (prefs !== null) {
            // 整段独立 try：服务端只验白名单键+合法 JSON 不验值域，本段任何异常只丢 prefs
            // 应用，绝不拖累下方 welcomeDone/beforeunload 会话建立门（FE-06 不被单点污染）
            try {
              const parts = splitPrefs(prefs);
              for (const [k, v] of Object.entries(parts.xterm)) {
                if (queryKeys.has(k)) {
                  continue; // query 优先（D-16——queryKeys 跳过机制即优先级实现）
                }
                // xterm setter 对值域非法值抛异常（cursorStyle 非 block/underline/bar、
                // lineHeight<1、scrollback<0——OptionsService._sanitizeAndValidateOption）；
                // 逐键 try/catch 与 query 通道构造路径容错对称（其构造器逐键隔离），
                // 单个非法键只丢该键；值内容不入日志（SEC-01 同纪律）
                try {
                  if (k === 'theme') {
                    // D-19 整体替换语义 + RESEARCH §Pitfall 3 合并修正：未指定键保留 wesh 调色板
                    // （部分 theme 不再把 tango 冲成 xterm 内建默认）；非对象 theme 值忽略 + warn
                    if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
                      term.options.theme = { ...defaultTheme, ...(v as Record<string, string>) };
                    } else {
                      console.warn('ignoring non-object theme pref');
                    }
                  } else {
                    // 白名单已保证键合法性，经一次收窄 cast；ITerminalOptions 运行时逐键赋值
                    // xterm 原生支持（RESEARCH §Pattern 6 核实注：OptionsService 通知各订阅方）
                    (term.options as unknown as Record<string, unknown>)[k] = v;
                  }
                } catch {
                  console.warn(`ignoring invalid pref value: ${k}`);
                }
              }
              // 全部应用完重算（D-13 + RESEARCH §Pitfall 7：fontSize/lineHeight/letterSpacing
              // 改变单元格尺寸，不 fit 则 cols/rows 与视口不符远端 TUI 画错）；
              // 既有 onResize→RESIZE 帧链路自动同步服务端
              fit.fit();
              // behavior 键写前端开关量（非 xterm 选项——禁止写 term.options，UI-SPEC 步骤 4）；
              // queryKeys 同跳过；typeof boolean 校验同 query 通道（服务端只验 JSON 不验类型）；
              // 位置必须在下方 welcomeDone/beforeunload 注册点之前——开关值在注册点已是最终态
              for (const [k, v] of Object.entries(parts.behavior)) {
                if (queryKeys.has(k)) {
                  continue;
                }
                if (typeof v !== 'boolean') {
                  console.warn(`ignoring non-boolean behavior pref: ${k}`);
                  continue;
                }
                if (k === 'resizeOverlay') {
                  resizeOverlayOn = v;
                } else if (k === 'confirmBeforeUnload') {
                  confirmBeforeUnloadOn = v;
                }
              }
              // OSC52（D-12：仅 --osc52 服务端可开启，只写不读——Warp CVE-2025-48725 教训）：
              // prefs.osc52===true 且 clipboardOK（非安全上下文不加载，OSC52 惰性）时加载；
              // readText 恒 resolve '' 而非 reject（RESEARCH §Pitfall 4：核心异步 OSC 链对
              // rejected promise rethrow 成 unhandled rejection，resolve 空串协议完整且零泄露
              // 同等安全）；writeText 同链路——页面失焦时 clipboard.writeText 以 NotAllowedError
              // 拒绝，不 catch 会被核心微任务硬抛成页面级未捕获异常，catch 告警后 resolve
              // （与选中复制失败静默同纪律）；provider 是构造第二参（§Pattern 4③；核心无内建
              // OSC52 handler——不加载则惰性无害）。签名以 addon-clipboard d.ts IClipboardProvider 为准
              if (prefs.osc52 === true && clipboardOK) {
                const writeOnly: IClipboardProvider = {
                  readText: (): Promise<string> => Promise.resolve(''),
                  writeText: (_sel, text): Promise<void> =>
                    navigator.clipboard.writeText(text).catch((e) => console.warn('osc52 clipboard write failed', e)),
                };
                term.loadAddon(new ClipboardAddon(undefined, writeOnly));
              }
            } catch {
              console.warn('discard prefs application of WELCOME frame');
            }
          }
          // FE-06：WELCOME 处理完成——会话建立门置位（浮层驱动自此响应 resize）；
          // 条件注册 beforeunload（默认开 ro 同启，D-18；上方 prefs/behavior 段已在
          // 本注册点之前完成开关翻转——从本 plan 起即为开关驱动形态）
          welcomeDone = true;
          if (confirmBeforeUnloadOn) {
            window.addEventListener('beforeunload', onBeforeUnload);
          }
        } catch {
          console.warn('discard malformed WELCOME frame');
        }
        break;
      }
      case ERROR: // D-06/D-07：暂存 {code,message}，onclose 按码分派时展示 message
        try {
          lastError = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
        } catch {
          console.warn('discard malformed ERROR frame');
        }
        break;
      default: // 未知 S→C 类型静默跳过（前向兼容，D-02 同纪律）
        break;
    }
  };

  // 启动聚焦：页面打开即键盘可用，无需先点击。
  // 顺序硬约束：线上首帧必须 Hello（D-02 携首尺寸）——fit 先行的尺寸由 Hello cols/rows
  // 承载（消除 80x24 首帧窗口），此间 onResize 触发的 sendResize 被 helloSent 门吞掉；
  // Hello 发出后窗口拖动经 onResize → sendResize 正常发送（握手已完成，协议合法）。
  sock.onopen = () => {
    opened = true;
    fit.fit();
    sock.send(
      concat(
        new Uint8Array([HELLO]),
        // ticket 为 undefined 时 JSON.stringify 自动省略该键——无认证模式与服务端
        // 跳过核销分支兼容（03-01 契约，proto.go HelloPayload.Ticket omitempty）
        enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols: term.cols, rows: term.rows, ticket })),
      ),
    );
    helloSent = true;
    term.focus();
  };

  sock.onerror = () => {
    // 握手失败（含第二客户端 409）；onclose 会随后再触发一次，showStatus 幂等
    if (!opened) {
      showStatus(
        'Unable to connect',
        'The wesh server is unreachable. It may have exited, or another client is already attached (wesh currently allows a single client).',
        'Check the shell where wesh is running, then',
      );
    }
  };

  // onclose 按 ev.code 分派人话文案（D-12①）——只认 code 不认 reason（库自动 1009 的
  // reason 是库内字符串不可控，RESEARCH Anti-Patterns）；1006 永不作为分派依据
  //（RFC6455 §7.4，无码异常断开落 default）。
  sock.onclose = (ev) => {
    // WS close 任意路径移除 beforeunload——含状态面板展示后与 auth_failed 重试前；
    // Session ended 后关页不再被拦截（D-18）；重试成功的新 WELCOME 会按开关重注册，无残留无双重
    window.removeEventListener('beforeunload', onBeforeUnload);
    // auth_failed 守卫（D-10）：ticket 60s 过期是正常场景（页面放置超 TTL）——
    // 静默重取 ticket 重试一次；重试再失败时 retriedAuth 已置位，落下方 switch 的
    // 1008 分支展示 lastError.message（非无限循环，T-03-25 缓解）
    if (lastError?.code === 'auth_failed' && !retriedAuth) {
      retriedAuth = true;
      lastError = null;
      void connect();
      return;
    }
    if (!opened) {
      showStatus(
        'Unable to connect',
        'The wesh server is unreachable. It may have exited, or another client is already attached (wesh currently allows a single client).',
        'Check the shell where wesh is running, then',
      );
      return;
    }
    switch (ev.code) {
      case 1000:
        showStatus(
          'Session ended',
          'The process exited and the wesh server has stopped.',
          'Start wesh again from your shell, then',
        );
        break;
      case 1008: // 策略违反（version_mismatch 等）——Error 帧 message 优先展示（D-07）
        showStatus(
          'Connection refused',
          lastError?.message ?? 'The server refused this connection.',
          'Start wesh again from your shell, then',
        );
        break;
      case 1009: // 超限（D-12① 不提 flag——本 phase 无可调 flag）
        showStatus(
          'Message too large',
          'Input exceeded the server message size limit and the connection was closed.',
          'Start wesh again from your shell, then',
        );
        break;
      case 1011:
        showStatus(
          'Server error',
          lastError?.message ?? 'The server hit an internal error.',
          'Start wesh again from your shell, then',
        );
        break;
      case 1013: // Phase 5 背压踢出占位路径——文案先行不占协议
        showStatus(
          'Disconnected',
          'The server asked this client to retry later.',
          'Start wesh again from your shell, then',
        );
        break;
      default: // 含 1002 协议错误与无码异常断开
        showStatus(
          'Connection lost',
          'The connection closed unexpectedly. In this phase the server exits when the connection drops.',
          'Start wesh again from your shell, then',
        );
        break;
    }
  };
}

void connect(); // 启动首连（替换顶层直连；auth_failed 重试经此同一入口）
