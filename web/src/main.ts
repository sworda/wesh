import '@xterm/xterm/css/xterm.css'; // xterm 必需样式，singlefile 内联
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';

// 帧常量与 internal/proto/proto.go 手工对齐（D-16，两侧注释互相指路）：
// '0' INPUT / '1' RESIZE / '0' OUTPUT / 'H' Hello / 'W' Welcome / 'E' Error；
// SUBPROTOCOL 同时是 WS 子协议 token 与 Hello.version 期望值（D-03，同源复用防双写漂移）
const OUTPUT = 0x30,
  INPUT = 0x30,
  RESIZE = 0x31,
  HELLO = 0x48,
  WELCOME = 0x57,
  ERROR = 0x45,
  SUBPROTOCOL = 'wesh.v1';

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
  theme: {
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
  },
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

let opened = false; // 是否曾成功 onopen——三态文案分派依据（UI-SPEC §Copywriting）
let helloSent = false; // Hello 首帧发出后才置位——此前 sendResize 门吞掉全部数据帧（防 RESIZE 抢跑被 1002 直关）
let lastError: { code: string; message: string } | null = null; // 最近一帧 Error{code,message}，onclose 展示用（D-07）

const ws = new WebSocket('ws://' + location.host + '/ws', [SUBPROTOCOL]); // D-03：wesh.v1 子协议建连
ws.binaryType = 'arraybuffer';

function concat(...parts: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) {
    out.set(p, off);
    off += p.length;
  }
  return out;
}

// S→C：按帧类型 switch 分派（与 server 握手段/数据面对称，D-01）
ws.onmessage = (ev) => {
  const buf = new Uint8Array(ev.data as ArrayBuffer);
  switch (buf[0]) {
    case OUTPUT: // 二进制帧直写（Uint8Array 二进制安全）
      term.write(buf.subarray(1));
      break;
    case WELCOME: {
      // D-14：ro 时键盘层面即不产生 onData（UX 层，真边界在服务端丢 INPUT）+ 标题 [ro] 前缀
      const w = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
      if (w.mode === 'ro') {
        term.options.disableStdin = true;
        document.title = '[ro] ' + document.title;
      }
      break;
    }
    case ERROR: // D-06/D-07：暂存 {code,message}，onclose 按码分派时展示 message
      lastError = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
      break;
    default: // 未知 S→C 类型静默跳过（前向兼容，D-02 同纪律）
      break;
  }
};

// C→S：键盘输入（CJK/IME 由 xterm 内部 composition 处理，onData 交付最终字符串）；
// 仅 OPEN 时发送——面板显示期间输入静默丢弃（UI-SPEC §Interaction Contract）
const enc = new TextEncoder();
term.onData((s) => {
  if (ws.readyState === WebSocket.OPEN) {
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
  if (ws.readyState !== WebSocket.OPEN) return;
  if (!Number.isInteger(cols) || cols <= 0 || !Number.isInteger(rows) || rows <= 0) return;
  ws.send(concat(new Uint8Array([RESIZE]), enc.encode(JSON.stringify({ cols, rows }))));
}
term.onResize(({ cols, rows }) => sendResize(cols, rows));
let timer: number | undefined;
window.addEventListener('resize', () => {
  clearTimeout(timer);
  timer = window.setTimeout(() => fit.fit(), 100);
});

// 启动聚焦：页面打开即键盘可用，无需先点击。
// 顺序硬约束：线上首帧必须 Hello（D-02 携首尺寸）——fit 先行的尺寸由 Hello cols/rows
// 承载（消除 80x24 首帧窗口），此间 onResize 触发的 sendResize 被 helloSent 门吞掉；
// Hello 发出后窗口拖动经 onResize → sendResize 正常发送（握手已完成，协议合法）。
ws.onopen = () => {
  opened = true;
  fit.fit();
  ws.send(
    concat(
      new Uint8Array([HELLO]),
      enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols: term.cols, rows: term.rows })),
    ),
  );
  helloSent = true;
  term.focus();
};

// #status 三态面板（UI-SPEC §Copywriting 逐字文案）：title/body + 提示行
// （提示行尾部为 accent 色 <a href="">Reload this page</a> 原地刷新链接）。
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

ws.onerror = () => {
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
ws.onclose = (ev) => {
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
