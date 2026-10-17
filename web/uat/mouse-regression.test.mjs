// 完整前端 DOM → WebSocket INPUT 回归；只模拟浏览器布局和外部服务。
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { JSDOM, VirtualConsole } from 'jsdom';

const html = readFileSync(new URL('../dist/index.html', import.meta.url), 'utf8');
const bundle = html.match(/<script[^>]*type="module"[^>]*>([\s\S]*?)<\/script>/)[1];

async function waitFor(predicate, label) {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (predicate()) return;
    await new Promise(resolve => setTimeout(resolve, 10));
  }
  throw new Error(`等待${label}超时`);
}

async function loadTerminal(t) {
  const virtualConsole = new VirtualConsole();
  const errors = [];
  virtualConsole.on('jsdomError', error => errors.push(error.message));
  const dom = new JSDOM('', { url: 'https://wesh.test/', runScripts: 'outside-only', pretendToBeVisual: true, virtualConsole });
  const { window } = dom;
  t.after(() => dom.window.close());
  window.TextEncoder = TextEncoder;
  window.TextDecoder = TextDecoder;
  window.structuredClone = structuredClone;
  window.fetch = async () => ({ ok: false, status: 404 });
  window.matchMedia = media => ({ matches: false, media, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} });
  Object.defineProperty(window, 'devicePixelRatio', { value: 1.5 });
  Object.defineProperty(window.navigator, 'maxTouchPoints', { value: 0 });

  // jsdom 没有布局引擎：固定现场几何 1896×1296、字符 8×16。
  const rect = { x: 0, y: 0, top: 0, left: 0, right: 1896, bottom: 1296, width: 1896, height: 1296 };
  window.HTMLElement.prototype.getBoundingClientRect = () => rect;
  for (const [property, value] of Object.entries({ offsetWidth: 1896, offsetHeight: 1296, clientWidth: 1896, clientHeight: 1296 }))
    Object.defineProperty(window.HTMLElement.prototype, property, { get: () => value, configurable: true });
  const metrics = text => ({ width: text.length * 8, fontBoundingBoxAscent: 13, fontBoundingBoxDescent: 3, actualBoundingBoxAscent: 13, actualBoundingBoxDescent: 3 });
  const ctx2d = { measureText: metrics, fillRect() {}, clearRect() {}, getImageData: (x, y, width, height) => ({ data: new Uint8ClampedArray(width * height * 4) }), putImageData() {}, createImageData: () => ({ data: new Uint8ClampedArray(0) }), setTransform() {}, drawImage() {}, save() {}, restore() {}, beginPath() {}, closePath() {}, rect() {}, clip() {}, fill() {}, translate() {}, scale() {}, rotate() {}, arc() {}, fillText() {}, strokeText() {} };
  window.HTMLCanvasElement.prototype.getContext = kind => kind === '2d' ? ctx2d : null;
  window.OffscreenCanvas = class {
    getContext(kind) { return kind === '2d' ? ctx2d : null; }
  };
  const getComputedStyle = window.getComputedStyle.bind(window);
  window.getComputedStyle = (element, pseudo) => {
    const style = getComputedStyle(element, pseudo);
    const getPropertyValue = style.getPropertyValue.bind(style);
    style.getPropertyValue = property => {
      if (property.startsWith('padding') && getPropertyValue(property) === '') return '0px';
      if (element.id === 'terminal' && property === 'width') return '1896px';
      if (element.id === 'terminal' && property === 'height') return '1296px';
      return getPropertyValue(property);
    };
    return style;
  };

  const frames = [];
  let socket;
  window.WebSocket = class {
    static OPEN = 1;
    readyState = 0;
    constructor() {
      socket = this;
      queueMicrotask(() => { this.readyState = 1; this.onopen?.(); });
    }
    send(bytes) {
      const frame = new Uint8Array(bytes);
      if (frame[0] === 48) frames.push(new TextDecoder().decode(frame.subarray(1)));
      if (frame[0] === 72) {
        const hello = JSON.parse(new TextDecoder().decode(frame.subarray(1)));
        this.receive('W', JSON.stringify({ mode: 'rw', session: 'shared', cols: hello.cols, rows: hello.rows }));
      }
    }
    receive(type, text) {
      const bytes = new TextEncoder().encode(type + text);
      this.onmessage?.({ data: bytes.buffer });
    }
    close() { this.readyState = 3; }
  };
  window.document.body.innerHTML = html.match(/<body[^>]*>([\s\S]*?)<\/body>/)[1];
  window.eval(bundle);
  await waitFor(() => socket?.readyState === 1, '前端握手');
  let marker = 0;
  const output = async text => {
    const title = `mouse-ready-${++marker}`;
    socket.receive('0', text + `\x1b]2;${title}\x07`);
    await waitFor(() => window.document.title === title, '终端输出解析');
    assert.deepEqual(errors, [], '页面执行不应有异常');
  };
  await output('\x1b[?1003h\x1b[?1006h');
  assert(window.document.querySelector('.xterm').classList.contains('enable-mouse-events'));
  frames.length = 0;
  const mouse = (type, buttons, { button = 0, x = 44, y = 71, shift = false, target } = {}) => {
    (target ?? window.document.querySelector('.xterm-screen')).dispatchEvent(new window.MouseEvent(type, { bubbles: true, cancelable: true, detail: 1, button, buttons, clientX: x, clientY: y, shiftKey: shift }));
  };
  return { window, frames, mouse, output };
}

test('漏掉一次释放后，后续每次点击仍发送按下和释放报告', async t => {
  const { frames, mouse } = await loadTerminal(t);
  mouse('mousedown', 1);
  frames.length = 0;
  for (let click = 0; click < 2; click++) {
    mouse('mousedown', 1);
    mouse('mouseup', 0);
  }
  assert.deepEqual(frames, ['\x1b[<0;6;5M', '\x1b[<0;6;5m', '\x1b[<0;6;5M', '\x1b[<0;6;5m']);
});

test('普通连续点击保持按下和释放的一一对应', async t => {
  const { frames, mouse } = await loadTerminal(t);
  for (let click = 0; click < 2; click++) {
    mouse('mousedown', 1);
    mouse('mouseup', 0);
  }
  assert.deepEqual(frames, ['\x1b[<0;6;5M', '\x1b[<0;6;5m', '\x1b[<0;6;5M', '\x1b[<0;6;5m']);
});

test('左右键交叠后仍报告部分释放、拖动和后续点击', async t => {
  const { frames, mouse } = await loadTerminal(t);
  mouse('mousedown', 1);
  mouse('mousedown', 3, { button: 2 });
  mouse('mouseup', 1, { button: 2 });
  mouse('mousemove', 1, { x: 52 });
  mouse('mouseup', 0, { x: 52 });
  mouse('mousedown', 1);
  mouse('mouseup', 0);
  assert.deepEqual(frames, ['\x1b[<0;6;5M', '\x1b[<2;6;5M', '\x1b[<2;6;5m', '\x1b[<32;7;5M', '\x1b[<0;7;5m', '\x1b[<0;6;5M', '\x1b[<0;6;5m']);
});

test('重复按下后拖动正常，最后释放清理 document 拖动监听', async t => {
  const { window, frames, mouse } = await loadTerminal(t);
  mouse('mousedown', 1);
  mouse('mousedown', 1);
  mouse('mousemove', 1, { x: 52, target: window.document });
  mouse('mouseup', 0, { x: 52, target: window.document });
  assert.deepEqual(frames, ['\x1b[<0;6;5M', '\x1b[<0;6;5M', '\x1b[<32;7;5M', '\x1b[<0;7;5m']);
  frames.length = 0;
  mouse('mousemove', 1, { x: 60, target: window.document });
  assert.deepEqual(frames, [], '最终释放后不应继续报告 document 拖动');
});

test('关闭鼠标协议不发送报告，重新开启后恢复点击', async t => {
  const { frames, mouse, output } = await loadTerminal(t);
  await output('\x1b[?1003l');
  mouse('mousedown', 1);
  mouse('mouseup', 0);
  assert.deepEqual(frames, []);
  await output('\x1b[?1003h');
  mouse('mousedown', 1);
  mouse('mouseup', 0);
  assert.deepEqual(frames, ['\x1b[<0;6;5M', '\x1b[<0;6;5m']);
});

test('Shift 强制选择不向应用发送鼠标报告', async t => {
  const { frames, mouse } = await loadTerminal(t);
  mouse('mousedown', 1, { shift: true });
  mouse('mouseup', 0, { shift: true });
  assert.deepEqual(frames, []);
  mouse('mousedown', 1);
  mouse('mouseup', 0);
  assert.deepEqual(frames, ['\x1b[<0;6;5M', '\x1b[<0;6;5m']);
});
