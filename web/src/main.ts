import '@xterm/xterm/css/xterm.css'; // xterm 必需样式，singlefile 内联
import { Terminal, type ITerminalOptions } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { ClipboardAddon, type IClipboardProvider } from '@xterm/addon-clipboard'; // 仅 WELCOME prefs osc52===true 时条件加载（D-12）
import { sanitizeTitle } from './lib/title';
import { parseQueryPrefs, splitPrefs, mergeTheme } from './lib/prefs';
import { backoffMs } from './lib/reconnect';

// 帧常量与 internal/proto/proto.go 手工对齐（D-16，两侧注释互相指路）：
// '0' INPUT / '1' RESIZE / '0' OUTPUT / 'H' Hello / 'W' Welcome / 'E' Error / 'X' EXIT；
// SUBPROTOCOL 同时是 WS 子协议 token 与 Hello.version 期望值（D-03，同源复用防双写漂移）。
// Hello 载荷 {version, cols, rows, ticket?}——ticket 为 Phase 3 认证核销一次性票（可选，
// 无认证模式省略该键，proto.go HelloPayload.Ticket omitempty 同形）；
// Welcome 载荷 {mode, cols, rows, prefs?}——cols/rows 为会话尺寸恒在键（G-05-1 方向 A，
// 05-10 三通道下发，proto.go WelcomePayload.Cols/Rows 恒序列化无 omitempty；
// 旧服务端缺席 = 不约束渲染）；prefs 为可选偏好下发字段（D-13 omitempty，proto.go WelcomePayload.Prefs 同形）；
// Error code 含 auth_failed（ticket 核销失败统一口径 D-10，proto.go ErrAuthFailed，前端据此静默重试一次）；
// EXIT 载荷 {exit_code, message}——子进程退出终结帧（Phase 6 D-08/D-09，proto.go Exit/ExitPayload
// 同形）；信号死亡 exit_code=-1；message 为服务端组文案唯一写口，前端直显不改写。
const OUTPUT = 0x30,
  INPUT = 0x30,
  RESIZE = 0x31,
  HELLO = 0x48,
  WELCOME = 0x57,
  ERROR = 0x45,
  EXIT = 0x58,
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
let lastExit: { exit_code: number; message: string } | null = null; // 最近一帧 EXIT{exit_code,message}，onclose 1000 展示用（D-10）
// 连接句柄提升为模块级：connect() 内赋值，onData/sendResize 等常驻接线引用当前连接；
// 首连前 fetch 窗口内为 null（此间用户敲击被 null 闸静默吞掉，不抛 TypeError）
let ws: WebSocket | null = null;
// connect() 代际序号（06-REVIEW CR-01）：每次 connect() 调用单调自增。D-04 既定
// 双在飞形态（online 事件/「Reconnect now」点击可在前一 attempt 的 fetch 飞行中再启
// attempt）下，较旧链迟到 resolve 时据 gen 复查识别自身陈旧——fetch 通道无 sock
// 可守卫（四 handler 的 sock !== ws 只覆盖 socket 建成后窗口），gen + welcomeDone
// 双检查即该通道的代际守卫（06-03 deviation #1 catch 半侧的 resolve 半侧补全）
let connectGen = 0;
let retriedAuth = false; // auth_failed 静默重试仅一次的门闩（D-10；无限重试会把 60s TTL 正常过期放大成重试风暴）
// ro 判定模块级化——标题写口/04-03 粘贴门/04-05 osc52 门三处共用（RESEARCH §Pattern 6 核实注）
let isRO = false;
// OSC52 一次性门闩（05 D-13 §OSC52 Boundary）：prefs.osc52===true 才加载 ClipboardAddon
// 且全程仅加载一次——升格 Welcome 重放 prefs 时防二次注册 OSC52 handler；
// ro 端永远收不到 osc52:true（服务端双档 blob 结构性不含该键）→ 永不加载，无需 ro 特判
let osc52Loaded = false;
// CORE-05 重连状态（页面级——osc52Loaded/retriedAuth 页面级门闩同族，connect()
// per-connection 重置块刻意不重置，IN-01 登记延伸）：
// reconnecting —— 重连循环单例门闩（Pitfall 5：onclose(1006)/offline/online 三触发源
// 共用幂等入口，断网瞬间事件相继到达不得启双循环）；attempt —— attempt 计数
//（首次自动重试 = attempt 1，初始连接不算 attempt，WELCOME 到达清零，C-9 语义）；
// reconnectTimer/retryAt/countdownTimer —— 退避等待定时器 / 到期时刻 / 面板 1Hz 倒计时
let reconnecting = false;
let attempt = 0;
let reconnectTimer: number | undefined;
let retryAt = 0;
let countdownTimer: number | undefined;

// 重连循环单例入口（Pitfall 5）——已在循环则幂等返回，三触发源共用
function startReconnect(): void {
  if (reconnecting) return;
  reconnecting = true;
  scheduleAttempt();
}

// 退避等待编排：面板显示等待期正文（attempt+1 = 即将到来的 attempt 序号）+ 退避定时器
// + 1Hz 倒计时（只写既有 #status-body 节点 textContent——面板不闪隐，
// UI-SPEC §Reconnect Panel Contract）；入口清旧定时器保恰好一次（双在飞 attempt 先后
// 失败重入本函数不叠加定时器——Pitfall 5 恰好一次纪律的机械核心）
function scheduleAttempt(): void {
  clearTimeout(reconnectTimer);
  clearInterval(countdownTimer);
  const delay = backoffMs(attempt);
  retryAt = Date.now() + delay;
  showStatus(RECONNECTING_TITLE, reconnectingWaitBody(attempt + 1, Math.ceil(delay / 1000)), RECONNECTING_HINT, {
    label: 'Reconnect now',
    onClick: runAttempt,
  });
  reconnectTimer = window.setTimeout(runAttempt, delay);
  countdownTimer = window.setInterval(() => {
    const remaining = Math.max(0, Math.ceil((retryAt - Date.now()) / 1000));
    document.getElementById('status-body')!.textContent = reconnectingWaitBody(attempt + 1, remaining);
  }, 1000);
}

// 立即 attempt（退避定时器触发 / 「Reconnect now」点击 / online 事件三调用点同一形态：
// 清当前等待立即试一次——不是新循环，D-04）；attempt++ 后显示在途正文，走完整 connect()
// 重入链（fetch ticket → Hello 核销——认证不绕行，结构性无静默豁免通道，T-06-03c）
function runAttempt(): void {
  clearTimeout(reconnectTimer);
  clearInterval(countdownTimer);
  attempt++;
  showStatus(RECONNECTING_TITLE, reconnectingNowBody(attempt), RECONNECTING_HINT, { label: 'Reconnect now', onClick: runAttempt });
  void connect();
}

// 循环终止（终态分派 / 重连成功两调用点）：退避清零 + 双定时器恰好一次清除 + 面板隐藏
function stopReconnect(): void {
  reconnecting = false;
  attempt = 0;
  clearTimeout(reconnectTimer);
  clearInterval(countdownTimer);
  document.getElementById('status')!.hidden = true;
}
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

// G-05-1 resize 链路四状态（connect() per-connection 重置块同批清零，IN-01 防漂移登记延伸）：
// sessionDims —— 最近一帧 Welcome 携会话尺寸；null = 旧服务端/未收到，渲染不约束（行为零漂移）。
// lastReported —— 最近一次实际上行的 fit 尺寸（sendResize 去重依据；只在真实发送后更新）。
// prevFit —— 上一次 refit 的 fit 尺寸（overlay 触发判定：窗口物理尺寸未变不闪浮层）。
let sessionDims: { cols: number; rows: number } | null = null;
let lastReported: { cols: number; rows: number } | null = null;
let prevFit: { cols: number; rows: number } | null = null;
// roNotified —— ro console 一次性提示门闩（运行期尺寸推送打破「ro Welcome 每 attach 仅一次」
// 天然无重复不变量后的等价物，WELCOME ro 分支接线）
let roNotified = false;

// FE-03 + CORE-02：resize 回路。window resize → 100ms debounce → refit() →
// RESIZE 帧（JSON {"cols","rows"} 恒为窗口 fit 尺寸，服务端钳制 [1,1000]）+ 约束渲染。
// 发送前防护：display:none 时 proposeDimensions 返回无效值（PITFALLS C10）。
function sendResize(cols: number, rows: number): void {
  // Hello 完成前禁发任何数据帧：onopen 首次 refit() 几乎必然走到 sendResize——
  // 不门住则 RESIZE 抢跑首帧，服务端握手段以 1002 frame_before_hello 直关
  if (!helloSent) return;
  // D-09 第一闸：ro 客户端不发 RESIZE。Hello 携首尺寸不受影响——helloSent 门先于
  // isRO 生效（isRO 仅在 WELCOME 到达后才可能为 true，彼时 Hello 已发出）；
  // 服务端忽略 ro RESIZE 为兜底第二闸（05-04 已落）
  if (isRO) return;
  if (ws === null || ws.readyState !== WebSocket.OPEN) return;
  if (!Number.isInteger(cols) || cols <= 0 || !Number.isInteger(rows) || rows <= 0) return;
  // 去重：与最近一次实际上行相同的尺寸不重发。去重键 = 上行 fit 尺寸（窗口物理尺寸，
  // 与被约束的渲染尺寸无关）；ro 期被 isRO 门拦截的调用不触 lastReported——升格后首次
  // refit 必真实上报（05-08 尺寸接管纠正链的保持机制）
  if (lastReported !== null && lastReported.cols === cols && lastReported.rows === rows) return;
  ws.send(concat(new Uint8Array([RESIZE]), enc.encode(JSON.stringify({ cols, rows }))));
  lastReported = { cols, rows };
}
let overlayTimer: number | undefined;
// refit 统一入口（G-05-1 前端半侧核心）：收编窗口监听/onopen/升格分支/prefs 段四个
// 原 fit.fit() 调用点。双概念拆分——上报尺寸 = fit.proposeDimensions()（窗口物理尺寸，
// 恒报全值驱动服务端 PTY 仲裁，永不是被约束的渲染尺寸，否则升格后尺寸接管纠正链断裂，
// G-05-1 设计约束 3）；渲染尺寸 = 逐轴 min(fit, sessionDims)。不采用「CSS 约束容器再 fit」
// 形态——那会使 proposeDimensions 返回被约束尺寸，上报/渲染两概念无法拆分。
// 渲染用 term.resize（ITerminal 稳定 API，FitAddon 自身底层同调用）；term.onResize
// 订阅已拆除——xterm 6 下 term.resize 唯一调用方是本函数（fit 插件仅经
// proposeDimensions 使用），无旁路触发面
function refit(): void {
  const d = fit.proposeDimensions();
  if (!d) return; // C10 守卫：display:none 时 proposeDimensions 返回 undefined
  // 渲染尺寸逐轴 min：窗口小于会话尺寸的轴按窗口渲染（裁剪语义不变，README 既有明示），
  // 大于的轴约束到会话矩形（G-05-1 修复面——同 cols 渲染同字节流，异尺寸双端逐屏一致）；
  // 超出面积为页面背景留白（纯布局零新 UI 组件，D-07）；sessionDims null（旧服务端）不约束
  const rCols = sessionDims ? Math.min(d.cols, sessionDims.cols) : d.cols;
  const rRows = sessionDims ? Math.min(d.rows, sessionDims.rows) : d.rows;
  if (term.cols !== rCols || term.rows !== rRows) {
    term.resize(rCols, rRows); // 变化才调——幂等，Welcome 推送重放零抖动
  }
  sendResize(d.cols, d.rows); // 上报恒为窗口 fit 尺寸；等值去重在 sendResize 内（lastReported）
  // FE-06 resize 浮层（D-17 语义钉死 = 窗口物理尺寸，本地 UX 辅助）：fit 尺寸未变的
  // 推送重放/约束变化不闪浮层（会话尺寸推送不改变本地窗口，浮层无由触发）；
  // welcomeDone && resizeOverlayOn 双门保持——onopen 初次 refit 不触发（浮层是会话辅助
  // 不是启动尺寸指示器）；ro 端窗口 resize 浮层同显（05-UI-SPEC R4 既定语义保持）
  const fitChanged = prevFit === null || prevFit.cols !== d.cols || prevFit.rows !== d.rows;
  prevFit = { cols: d.cols, rows: d.rows };
  if (!welcomeDone || !resizeOverlayOn || !fitChanged) return;
  const overlay = document.getElementById('resize-overlay')!;
  overlay.textContent = `${d.cols}x${d.rows}`; // 服务端钳制 1000×1000 → 最长 9 字符无溢出（UI-SPEC §Resize Overlay Spec）
  overlay.hidden = false;
  overlay.style.opacity = '1';
  clearTimeout(overlayTimer);
  overlayTimer = window.setTimeout(() => {
    overlay.style.opacity = '0'; // 静止 600ms 后置 0，经 transition 200ms 淡出
  }, 600);
}
let timer: number | undefined;
window.addEventListener('resize', () => {
  clearTimeout(timer);
  timer = window.setTimeout(() => refit(), 100);
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

// 05-UI-SPEC §Copywriting 同源文案常量（R1/R3 修订落点）：
// C-4 Unable to connect 正文三处同源（fetch catch / onerror !opened / onclose !opened）——
// 多客户端化使旧版"另一客户端已连接（单客户端）"表述事实错误（R1）；title 与 hintPrefix 不变
const UNREACHABLE_BODY =
  'The wesh server is unreachable. It may have exited, or it is refusing new connections (for example, because it is full).';
// C-6 共用提示行前缀（1008/1009/1011 三条单写口防漂移，R3）——服务端不再随断开退出，
// "先重启服务端"不再是首要建议；Session ended (1000) 的提示行语义仍精确为真，不在此列
const HINT_RESTART = 'If the problem persists, restart wesh from your shell, then';

// C-9 Reconnecting 面板文案（06-UI-SPEC §Copywriting 逐字契约，D-03/D-11）——
// 三处同源单写口纪律（05-08 C-4/C-6 常量化先例）：scheduleAttempt 初显 /
// countdown 1Hz 更新 / runAttempt 在途共用以下常量与模板函数
const RECONNECTING_TITLE = 'Reconnecting';
const RECONNECTING_HINT = 'If the server has exited, restart it from your shell. To skip the wait,';
// 等待期正文（1Hz 倒计时更新）：N = 即将到来的 attempt 序号，S = 剩余秒数
function reconnectingWaitBody(n: number, s: number): string {
  return `The connection was lost. Retrying in ${s}s (attempt ${n}).`;
}
// attempt 在途正文（定时器触发或手动点击后 connect() 飞行中）
function reconnectingNowBody(n: number): string {
  return `The connection was lost. Retrying now (attempt ${n})...`;
}

// #status 三态面板（UI-SPEC §Copywriting 逐字文案）：title/body + 提示行
// （提示行尾部为 accent 色动作链接）。R3/OQ2 定稿：动作链接经第四可选参 action
// 参数化（label + onClick）——缺省保持 'Reload this page' + location.reload()
// 逐字现状（全部既有调用点零改动、渲染逐字节不变）；Reconnecting 面板传
// 'Reconnect now' + runAttempt（C-9）。
// 幂等：textContent 赋值先清空子节点再重建，onerror/onclose 双触发不重复渲染。
function showStatus(title: string, body: string, hintPrefix: string, action?: { label: string; onClick: () => void }): void {
  document.getElementById('status-title')!.textContent = title;
  document.getElementById('status-body')!.textContent = body;
  const hint = document.getElementById('status-hint')!;
  hint.textContent = hintPrefix + ' ';
  const a = document.createElement('a');
  a.href = '';
  a.textContent = action?.label ?? 'Reload this page';
  a.addEventListener('click', (e) => {
    e.preventDefault(); // 两态统一阻断空 href 的隐式导航（部分情境下不可靠）
    if (action !== undefined) {
      action.onClick();
    } else {
      // 显式 reload——不依赖空 href 的隐式导航行为
      location.reload();
    }
  });
  hint.appendChild(a);
  hint.appendChild(document.createTextNode('.'));
  document.getElementById('status')!.hidden = false;
}

// 认证感知连接流程（D-02 前端半侧）：fetch POST /api/attach 取一次性 ticket →
// 建 WS → Hello{version,cols,rows,ticket?}。ticket 只存本函数闭包变量与 Hello 载荷——
// 禁止写入 URL query/localStorage/console（T-03-24 泄漏面红线）。
// 分享链接进入（05 D-01/D-03）：token 红线为 ticket 同款延伸——只存本函数闭包变量
// 与 POST body，禁 console/localStorage/sessionStorage/任何日志调用；禁经 URL 重写
// API 剥离 URL token（D-03 路径段是分享/书签契约，1013 后手动刷新必须凭原 URL
// 重新 attach——剥离会使 D-10 手动刷新入口失效；验收断言以源码零调用形态锁定）。
async function connect(): Promise<void> {
  const gen = ++connectGen; // 本链代际序号（CR-01）——每个 await 挂起点后、行动前复查
  // 每次尝试重置 per-connection 状态——auth_failed 重试不携带上次连接残留
  opened = false;
  helloSent = false;
  lastError = null;
  lastExit = null; // IN-01 防漂移登记同款——auth_failed 重试/重连不携带上连接的 EXIT 暂存
  // isRO/welcomeDone 同属 per-connection（IN-01 防漂移登记，Phase 6 自动重连落地前提）；
  // osc52Loaded/retriedAuth 与 reconnecting/attempt 等重连循环状态为页面级门闩，刻意不重置
  isRO = false;
  welcomeDone = false;
  // G-05-1 resize 四状态同批清零（IN-01 延伸）——auth_failed 重试/未来 Phase 6 重连
  // 不携带上连接的残留尺寸约束、去重基线、overlay 基线与 ro 提示门闩
  sessionDims = null;
  lastReported = null;
  prevFit = null;
  roNotified = false;

  // ^/s/{token}/$ 提取（无尾斜杠由服务端 301 补斜杠，前端无需兼容——05 R-05）；
  // 前端不解析不分支 token 种类——ro/rw 判定唯一来源是 Welcome.mode（05 D-01）
  const shareMatch = location.pathname.match(/^\/s\/([^/]+)\/$/);
  const shareToken = shareMatch ? shareMatch[1] : undefined;

  let ticket: string | undefined;
  try {
    // credentials 默认 'same-origin'：浏览器 HTTP auth 缓存条目随同源 fetch 自动附带
    // （D-02 成立前提——先导航 GET / 弹原生 Basic 框缓存凭据；A2 假设，UAT 必验）；
    // 携 token 时 POST body 上送（OQ1：token 通道与认证模式正交——无认证模式同样走
    // 本分支，服务端 /api/attach 仅当 body 携 token 时非 404）；无 token 保持空 body 现状
    const resp = await fetch(
      '/api/attach',
      shareToken === undefined
        ? { method: 'POST' }
        : {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token: shareToken }),
          },
    );
    // 代际复查①（CR-01）：fetch resolve 后、状态码分派前——本链已被更新的 connect()
    // 取代（gen 过期：双击/online 竞速）或健康会话已建立（welcomeDone，06-03
    // deviation #1 同款代际标记）时静默丢弃返回：迟到链不得落 401/429/503 等终态
    // 面板覆盖健康会话/更新链 UI（catch 通道已有同款守卫，本行为 resolve 通道的对称
    // 收口）
    if (gen !== connectGen || welcomeDone) return;
    if (resp.ok) {
      ticket = (await resp.json()).ticket;
    } else if (resp.status === 404) {
      // 无认证模式（--no-auth/loopback 裸跑）探测信号：跳过 ticket 直连 WS
      // （RESEARCH Pattern 1 决策；服务端无凭据时 /api/attach 显式注册 404）；
      // 仅未携 token 时可到此分支（OQ1：携 token 时无认证模式同样非 404 签发）
      ticket = undefined;
    } else if (resp.status === 401 && shareToken !== undefined) {
      // C-3 专版（R-05）：携 token 的 401 = 分享链接无效/过期——前端自知本次请求
      // 携 token，不向攻击者泄露任何其本不知道的信息（无 oracle 纪律不约束前端文案）
      // 重连上下文的终态分派（UI-SPEC §Reconnect Dispatch 循环终止条件逐字契约）：
      // 各专版面板分支统一先终止循环再落面板（面板文案逐字既有）；404 探测直连不在
      // 其列——链路继续走 WS，成功终止由 WELCOME 到达点的 stopReconnect 承载
      if (reconnecting) stopReconnect();
      showStatus(
        'Invalid share link',
        'This share link is invalid or has expired. Share links are regenerated each time wesh restarts.',
        'Ask the operator for a new link, then',
      );
      return;
    } else if (resp.status === 429) {
      if (reconnecting) stopReconnect(); // 终态循环终止（同上逐字契约）
      showStatus(
        'Too many attempts',
        'Too many failed authentication attempts. The server is temporarily refusing new attempts.',
        'Wait a moment, then',
      );
      return;
    } else if (resp.status === 503) {
      // C-2 专版（OQ2）：/api/attach 容量早闸——任意请求满员同此分支
      if (reconnecting) stopReconnect(); // 终态循环终止（同上逐字契约）
      showStatus(
        'Server is full',
        'The server has reached its maximum number of attached clients.',
        'Wait for a slot to free up, then',
      );
      return;
    } else {
      // 401 未携 token 及其余非 ok 状态同口径：通用认证失败，不细分（无 oracle 纪律延伸到前端文案）。
      // fetch 的 401 不弹浏览器原生登录框（Pitfall 6 平台行为）——引导重新加载页面，
      // 重新导航触发浏览器原生 Basic 弹窗（不自建登录表单，D-02 零新 UI 纪律）。
      if (reconnecting) stopReconnect(); // 终态循环终止（同上逐字契约）
      showStatus(
        'Authentication failed',
        'The server rejected your credentials. Reloading the page re-opens the browser credential prompt.',
        'Check your credentials, then',
      );
      return;
    }
  } catch {
    // 重连上下文：fetch throw = 网络不可达/服务端已退出不可区分（D-11）——留在循环，
    // Reconnecting 面板持续，不被 'Unable to connect' 覆盖（Pitfall 7 面板保护）
    if (reconnecting) {
      scheduleAttempt();
      return;
    }
    // 陈旧 fetch 迟到失败（双在飞 attempt 中较慢者——online 事件/手动点击可在前一
    // attempt 的 fetch 飞行中再启 attempt，D-04 既定形态）：新会话已建立时不得用
    // 'Unable to connect' 覆盖健康会话（Pitfall 6 同族代际污染——fetch 通道无 sock
    // 可代际守卫，welcomeDone 即「新会话已建立」代际标记）
    if (welcomeDone) return;
    showStatus('Unable to connect', UNREACHABLE_BODY, 'Check the shell where wesh is running, then');
    return;
  }

  // 代际复查②（CR-01）：resp.json() 二次挂起后、提交句柄前最后复查（复查①到本点间
  // 仍可能被更新链取代）。通过者提交前关闭被取代的旧 socket：重连窗口期旧 socket
  // 可能已 onopen 发出 Hello 完成 attach 而 WELCOME 尚未处理（welcomeDone 仍 false），
  // 不 close 则成幽灵连接——浏览器协议栈透明回 pong，服务端 pinger/pongTimeout 永不
  // 触发，永久占用注册表槽位与 owner 身份，本链新连接反被降级 ro（CR-01 场景 B 双击
  // 形态）。健康会话不会到此（两处复查的 welcomeDone 先行拦截）——ws?.close() 只命中
  // null/已死 socket（幂等空转）或在飞残骸（本意）；被关旧 socket 的迟到事件经
  // sock !== ws 守卫静默空转，beforeunload 监听不误拆
  if (gen !== connectGen || welcomeDone) return;
  ws?.close();
  // scheme 按页面协议选 ws/wss——https 页面下必须 wss（TLS 部署可连，03-04 伺服形态）
  ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws', [SUBPROTOCOL]); // D-03：wesh.v1 子协议建连
  const sock = ws; // 闭包内引用本连接的确定句柄（TS 对模块级可空 let 不做闭包收窄）
  sock.binaryType = 'arraybuffer';

  // S→C：按帧类型 switch 分派（与 server 握手段/数据面对称，D-01）
  sock.onmessage = (ev) => {
    // 代际守卫（Pitfall 6，重连落地起为必需闸）：重连引入「旧 socket 未死透 + 新 socket
    // 已建立」双连接窗口——stale 代际事件不得触碰新会话状态（今日单连接生命周期下该判定恒真）
    if (sock !== ws) return;
    const buf = new Uint8Array(ev.data as ArrayBuffer);
    switch (buf[0]) {
      case OUTPUT: // 二进制帧直写（Uint8Array 二进制安全）
        term.write(buf.subarray(1));
        break;
      case WELCOME: {
        // D-14：ro 时键盘层面即不产生 onData（UX 层，真边界在服务端丢 INPUT）
        // 畸形 JSON 负载丢弃该帧——事件处理器抛异常只丢本帧，但 WELCOME 丢失会让 ro 门失效
        // Welcome 幂等矩阵（重复/推送 Welcome 全链零副作用，G-05-1 推送通道不放大）：
        // 尺寸推送重放同值 → term.resize 变化守卫跳过 + sendResize lastReported 去重拦截 +
        // overlay fitChanged 门拦截；升格 Welcome（rw）→ 尺寸键校验 → mode 分支 → 统一 refit 链；
        // prefs 重放 → queryKeys 跳过 + osc52Loaded 门闩（既有）；welcomeDone/beforeunload
        // 重复注册 → DOM 同类型同 listener addEventListener 去重（既有）
        try {
          const w = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
          // G-05-1 会话尺寸键处理（05-10 契约：attach/升格/运行期推送三通道同形恒携）：
          // 成对校验——任一键出现即视为服务端新形态，两键均须 [1,1000] 正整数才接受；
          // 任一键缺失/非法 → console.warn 并保持旧 sessionDims（D-16 容错纪律：非法输入
          // 不得使终端不可用，与 invalid query pref 同款静默降级方向；T-05G-04 缓解——
          // term.resize 永收合法正整数）。两键均缺席 = 旧服务端 → 不动 sessionDims
          //（恒 null → 渲染=fit，行为与改造前逐字节一致）
          if ('cols' in w || 'rows' in w) {
            if (
              typeof w.cols === 'number' && Number.isInteger(w.cols) && w.cols >= 1 && w.cols <= 1000 &&
              typeof w.rows === 'number' && Number.isInteger(w.rows) && w.rows >= 1 && w.rows <= 1000
            ) {
              sessionDims = { cols: w.cols, rows: w.rows };
            } else {
              console.warn('ignoring invalid session dims in WELCOME frame');
            }
          }
          if (w.mode === 'ro') {
            isRO = true;
            term.options.disableStdin = true;
            // 经单一写口补 [ro] 前缀——remoteTitle 不含前缀，auth_failed 重试再收
            // WELCOME 不产生 '[ro] [ro] …' 双重前缀（D-02 前缀恒最前的回归意图）
            setTitle();
            // ro 模式一次性 console 反馈（review #4 缓解链：输入不发送 + 裁剪风险 +
            // 恢复指引三要素——旁观者与降级递补者同一条，前端不加区分既有纪律）。
            // console 非可视组件不违 D-07（[ro] 标题前缀 + disableStdin 仍是仅有的模式
            // 指示）；运行期尺寸推送使 ro Welcome 每连接可多次到达——一次性语义改由
            // roNotified 门闩承载（原「ro Welcome 每 attach 仅一次」天然无重复不变量的
            // 等价物）；串内零 token 零动态内容（T-03-24 红线延伸）。文案语义复核：
            // 约束形态下「窗口小于会话尺寸可能裁剪」依然为真（min 逐轴——小窗口轴
            // 仍裁剪），文案零改动成立
            if (!roNotified) {
              roNotified = true;
              console.info('wesh: read-only mode — input is not sent; if your window is smaller than the session size, output may appear clipped (reattach to recover)');
            }
          } else if (w.mode === 'rw') {
            // 升格 rw 分支（05 §RO Mode & Promotion Contract 五步之 1-3——第 4 步 prefs
            // 应用段与第 5 步 beforeunload 条件重注册由下方既有流程照常执行承载）。
            // 握手 Welcome（attach 即 rw）走同一分支无害：isRO 本就 false，重复赋 false
            // 与重复 refit 均幂等；降级路径不存在不实现（owner 只在位到断线，D-06）
            isRO = false;
            term.options.disableStdin = false;
            setTitle(); // 单一写口去 [ro] 前缀（remoteTitle 不含前缀，04 契约既定）
          }
          // 统一 refit（约束应用唯一入口，ro/rw 两分支统一覆盖——ro 端 attach 即按会话
          // 尺寸约束渲染，正是 G-05-1 用户场景「宽端旁观」的修复动作；rw attach/升格同理）。
          // 顺序硬约束：sessionDims 先赋值（上方尺寸键处理）→ isRO/disableStdin/setTitle
          // 居中 → refit 最后——refit 时 sessionDims 已是新会话尺寸（升格情形 = 本端
          // Hello 登记尺寸），min(fit, session) 自然解除约束回到窗口渲染（G-05-1 设计
          // 约束 4「先解除约束再 fit」的落地形态）。升格时 refit 内 sendResize(当前 fit)
          // 经 lastReported 去重判定——ro 期上报被 isRO 门拦截未记账，ro 期拖过窗口的端
          // 此处真实上行纠正服务端 cand.dims 瞬态偏差（05-08 尺寸接管纠正链保持，R-09；
          // 服务端 recalcNow 随后推送收口，05-10）
          refit();
          // FE-07 prefs 应用段（UI-SPEC §Prefs Contract 步骤 2-4 逐字顺序：
          // xterm 键 → refit() → behavior 键 → osc52 条件加载）；
          // prefs 缺省（非对象）则整段跳过——nil 兼容（omitempty 缺席即无下发）；
          // 升格 Welcome 携 rw 档 prefs 重放本段：queryKeys 跳过机制幂等可重入，
          // xterm 键/behavior 键/theme 重放同值无害，osc52 由 osc52Loaded 门闩防二次加载
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
              // 改变单元格尺寸，不 refit 则 cols/rows 与视口不符远端 TUI 画错）；
              // refit 内 sendResize 恒报窗口 fit 尺寸自动同步服务端
              refit();
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
              // OSC52 handler——不加载则惰性无害）。签名以 addon-clipboard d.ts IClipboardProvider 为准；
              // osc52Loaded 一次性门闩：升格 Welcome 重放 prefs 防二次注册 OSC52 handler
              if (prefs.osc52 === true && clipboardOK && !osc52Loaded) {
                osc52Loaded = true;
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
          // CORE-05 重连成功点（D-05，UI-SPEC §Reconnect Success Contract）：WELCOME 到达 =
          // 重连成功唯一判定（WS 建连成功但握手未完成不得清零退避或隐藏面板——不得伪装接回）。
          // 顺序：stopReconnect（退避清零 + 面板隐藏）→ 清屏（不保留旧 buffer——重连窗口期
          // 错过的输出形成断层，增量重绘有 G-05-1 同源花屏风险；服务端 attach 路径既有
          // SIGWINCH 强制重绘恒触发，server.go:752 挂点零改动）→ 下方 beforeunload 按开关
          // 重注册既有代码照常（P4 D-18 先例）。清屏操作与 osc52Loaded 门闩无耦合
          //（buffer 清操作不触模块级页面门闩）；标题保持最后 remoteTitle 直到下次
          // OSC 2/重绘（P5 D-12——不主动重置）
          if (reconnecting) {
            stopReconnect();
            term.clear();
          }
          // FE-06：WELCOME 处理完成——会话建立门置位（浮层驱动自此响应 resize）；
          // 条件注册 beforeunload（默认开 ro 同启，D-18；上方 prefs/behavior 段已在
          // 本注册点之前完成开关翻转——从本 plan 起即为开关驱动形态）；
          // 升格 Welcome 重放到此同参重复注册——DOM 规范对同类型同 listener 的
          // addEventListener 去重，幂等无泄漏（05 RESEARCH Pattern 9 核实）
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
      case EXIT: // D-09/D-10：暂存 {exit_code,message}，onclose 1000 分支正文显示 message
        try {
          lastExit = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
        } catch {
          console.warn('discard malformed EXIT frame');
        }
        break;
      default: // 未知 S→C 类型静默跳过（前向兼容，D-02 同纪律）
        break;
    }
  };

  // 启动聚焦：页面打开即键盘可用，无需先点击。
  // 顺序硬约束：线上首帧必须 Hello（D-02 携首尺寸）——refit 先行的尺寸由 Hello cols/rows
  // 承载（消除 80x24 首帧窗口），此间 refit 内的 sendResize 被 helloSent 门吞掉；
  // Hello 发出后窗口拖动经 refit → sendResize 正常发送（握手已完成，协议合法）。
  sock.onopen = () => {
    if (sock !== ws) return; // 代际守卫（Pitfall 6）——stale socket 的迟到 onopen 不得驱动新会话
    opened = true;
    refit();
    sock.send(
      concat(
        new Uint8Array([HELLO]),
        // ticket 为 undefined 时 JSON.stringify 自动省略该键——无认证模式与服务端
        // 跳过核销分支兼容（03-01 契约，proto.go HelloPayload.Ticket omitempty）
        enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols: term.cols, rows: term.rows, ticket })),
      ),
    );
    helloSent = true;
    // Hello 载荷即首次尺寸上报——同步 lastReported 防握手 Welcome 到达后的 refit 把
    // 等值尺寸作为「变化」重发一帧冗余 RESIZE（线序零漂移纪律：改造前握手后无此帧）
    lastReported = { cols: term.cols, rows: term.rows };
    term.focus();
  };

  sock.onerror = () => {
    if (sock !== ws) return; // 代际守卫（Pitfall 6）
    // 重连上下文不显示任何面板（Pitfall 7 面板保护）——onclose 随后来临分派
    //（1006 → scheduleAttempt 留循环；带码关闭 → 终态专版面板）
    if (reconnecting) return;
    // 握手失败（含 WS 握手阶段满员 503——早闸后竞态窗口浏览器不暴露握手状态码，
    // 落本通用文案，OQ2 裁决注记）；onclose 会随后再触发一次，showStatus 幂等
    if (!opened) {
      showStatus('Unable to connect', UNREACHABLE_BODY, 'Check the shell where wesh is running, then');
    }
  };

  // onclose 按 ev.code 分派人话文案（D-12①）——只认 code 不认 reason（库自动 1009 的
  // reason 是库内字符串不可控，RESEARCH Anti-Patterns）；1006 = 重连唯一触发码
  //（浏览器本地合成码，永不出现于线上——RFC6455 §7.4，proto.go 关闭码纪律不变）；
  // 其余码分派语义不变。
  sock.onclose = (ev) => {
    // 代际守卫（Pitfall 6）——stale socket 不得拆新连接刚注册的 beforeunload 监听（R4），
    // 不得用迟到的 onclose 覆盖新会话状态/面板
    if (sock !== ws) return;
    // WS close 任意路径移除 beforeunload——含状态面板展示后与 auth_failed 重试前；
    // Session ended 后关页不再被拦截（D-18）；重试成功的新 WELCOME 会按开关重注册，无残留无双重
    window.removeEventListener('beforeunload', onBeforeUnload);
    // auth_failed 守卫（D-10）：ticket 60s 过期是正常场景（页面放置超 TTL）——
    // 静默重取 ticket 重试一次；重试再失败时 retriedAuth 已置位，落下方 switch 的
    // 1008 分支展示 lastError.message（非无限循环，T-03-25 缓解）；
    // 携 token 流程同样适用——重试经 connect() 同一入口重新 POST 携 token
    if (lastError?.code === 'auth_failed' && !retriedAuth) {
      retriedAuth = true;
      lastError = null;
      void connect();
      return;
    }
    // 重连上下文分派（Pitfall 7 面板保护的 onclose 半侧）：再 1006 = 本次 attempt 失败——
    // scheduleAttempt 留在循环；带码关闭 = 服务端明确语义（auth_failed 重试耗尽后的
    // 1008 等）——终止循环，落下方既有专版面板分派逐字不变
    if (reconnecting && ev.code === 1006) {
      scheduleAttempt();
      return;
    }
    if (reconnecting) {
      stopReconnect();
    }
    if (!opened) {
      showStatus('Unable to connect', UNREACHABLE_BODY, 'Check the shell where wesh is running, then');
      return;
    }
    switch (ev.code) {
      case 1000:
        showStatus(
          'Session ended',
          // R2：正文 = EXIT 帧服务端组文案（退出码/信号人话，D-09/D-10）；未收 EXIT
          //（旧服务端/异常路径）回退既有硬编码文案逐字不变（前向兼容 P2 D-02）
          lastExit?.message ?? 'The process exited and the wesh server has stopped.',
          'Start wesh again from your shell, then',
        );
        break;
      case 1008: // 策略违反（version_mismatch 等）——Error 帧 message 优先展示（D-07）
        showStatus(
          'Connection refused',
          lastError?.message ?? 'The server refused this connection.',
          HINT_RESTART,
        );
        break;
      case 1009: // 超限（D-12① 不提 flag——本 phase 无可调 flag）
        showStatus(
          'Message too large',
          'Input exceeded the server message size limit and the connection was closed.',
          HINT_RESTART,
        );
        break;
      case 1011:
        showStatus(
          'Server error',
          lastError?.message ?? 'The server hit an internal error.',
          HINT_RESTART,
        );
        break;
      case 1013: // C-1 专版（05 D-10/R-10）：慢消费者背压踢出。只认 ev.code 不渲染
        // reason 内容（slow_consumer 是机器串，渲染远端内容是伪造钓鱼面）；不做任何
        // 自动重连——后台标签页重连→再被踢循环比手动刷新更差（Phase 6 CORE-05 边界）；
        // beforeunload 移除联动见本函数入口；URL token 保留使手动刷新可重新 attach
        showStatus(
          'Disconnected',
          'This client was disconnected because it could not keep up with the session output. The session itself is unaffected.',
          'To reattach from the latest output,',
        );
        break;
      case 1006: // CORE-05 触发谓词（D-01 显式判定——不是 default 桶）：仅 opened 已置位的
        // 已建立会话异常断开进重连（上方 !opened 分支先行收口首连失败）；1002 协议错误
        // 等带码关闭留 default 桶 C-5 手动面板逐字不变；1013 被踢维持手动刷新（P5 D-10——
        // 自动重连只会再被踢且后台标签页循环放大流量）
        startReconnect();
        break;
      default: // C-5（R2 改写；R1 收窄——1006 已显式抽出进重连，残留 = 1002 协议错误
        // 等带码关闭）——客户端断开不再触发服务端退出，
        // 旧提示行"In this phase the server exits…"语义已死
        showStatus(
          'Connection lost',
          'The connection closed unexpectedly.',
          'The session may still be running. To reattach,',
        );
        break;
    }
  };
}

void connect(); // 启动首连（替换顶层直连；auth_failed 重试经此同一入口）

// CORE-05 断线检测双触发（D-04）：浏览器 online/offline 事件为快路径提示层
//（OS 级网络断开/恢复秒级感知），onclose(1006) 为权威信号；黑洞场景
//（无 RST 无事件）退化为 TCP 超时后重连——风险接受（CONTEXT D-04 明示）
window.addEventListener('offline', () => {
  // 已在循环幂等返回（Pitfall 5）；未建立会话的初连窗口不启动——防与在飞 connect 双连接
  if (reconnecting || !welcomeDone) return;
  // 连接健康等 onclose 权威信号（loopback 开发场景 offline 不断 loopback TCP）
  if (ws !== null && ws.readyState === WebSocket.OPEN) return;
  startReconnect();
});
window.addEventListener('online', () => {
  // 清当前等待定时器立即试一次——不是新循环（D-04）；runAttempt 内清双 timer + attempt++
  if (reconnecting) runAttempt();
});
