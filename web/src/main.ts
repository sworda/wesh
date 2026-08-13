import '@xterm/xterm/css/xterm.css'; // xterm 必需样式，singlefile 内联
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';

// 帧常量与 internal/proto/proto.go 手工对齐（D-16）：'0' INPUT / '1' RESIZE / '0' OUTPUT
const OUTPUT = 0x30,
  INPUT = 0x30,
  RESIZE = 0x31;

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

const ws = new WebSocket('ws://' + location.host + '/ws');
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

// S→C：OUTPUT 二进制帧直写（Uint8Array 二进制安全）
ws.onmessage = (ev) => {
  const buf = new Uint8Array(ev.data as ArrayBuffer);
  if (buf[0] === OUTPUT) {
    term.write(buf.subarray(1));
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

// 启动聚焦：页面打开即键盘可用，无需先点击
ws.onopen = () => {
  opened = true;
  fit.fit();
  sendResize(term.cols, term.rows);
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

ws.onclose = (ev) => {
  if (!opened) {
    showStatus(
      'Unable to connect',
      'The wesh server is unreachable. It may have exited, or another client is already attached (wesh currently allows a single client).',
      'Check the shell where wesh is running, then',
    );
  } else if (ev.code === 1000) {
    showStatus(
      'Session ended',
      'The process exited and the wesh server has stopped.',
      'Start wesh again from your shell, then',
    );
  } else {
    showStatus(
      'Connection lost',
      'The connection closed unexpectedly. In this phase the server exits when the connection drops.',
      'Start wesh again from your shell, then',
    );
  }
};
