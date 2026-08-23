// Phase 6 CORE-05 重连状态机前端逻辑面自动化 UAT（jsdom + Node 原生 WebSocket/fetch）。
//
// 定位：06-03 重连状态机改造（面板保护/代际守卫/单例循环）全部风险点的可执行断言——
// jsdom 加载真实构建产物 web/dist/index.html，注入 SpyWebSocket（合成 CloseEvent 能力）
// 连接真实 spawn 的 wesh 实例端到端断言（headless 硬约束，根 CODEBUDDY.md 分层策略层 3）。
// 协议面「杀 WS → 重连接回原 PTY」由 phase06.mjs（06-06）覆盖；本脚本覆盖逻辑面：
//   D1 1006 重连全链（Reconnecting 面板 C-9 → 退避自动重连 → 清屏 → beforeunload 重注册）
//   D2/D3 不触发边界（1002/1013/1008 → 各专版手动面板 + 守候窗零新连接）
//   D4 双触发幂等（offline + onclose(1006) 相继到达 → 单循环，Pitfall 5）
//   D5 Reconnect now 手动入口（点击即跳过等待立即 attempt，循环不终止）
//   D6 代际守卫（旧 socket 迟到 onclose 零污染新会话，Pitfall 6）
//   D7 EXIT 帧全链（真实服务端 exit 7 → Session ended 正文逐字 + 进程退出码）
//   D8 online 快路径（清等待立即 attempt，D-04）
//   D9 真实断网栈豁免场景（skipped+reason，指向 06-UAT.md 人工清单）
//   D10 CR-01 代际守卫 fetch 半侧（双在飞 attempt 较旧链迟到成功不踩占健康连接）
//
// 本文件夹具（phase05-dom.mjs loadTerminal 形态逐字复用 + 两件延伸）：
//   - SpyWebSocket.synthClose(code)：合成 CloseEvent 驱动 onclose 分派（06-RESEARCH A2
//     兑现——jsdom 25 CloseEvent 构造器实测可用；若构造受阻回退 Event + code 赋值，
//     A2 注释登记）。先取存 this.onclose 处理器并置 null（抑制随后真实 close 的 1000
//     事件混入断言面），再 try{ this.close() }catch{}（真实底层连接尽力关闭），最后
//     以该处理器调用合成事件；处理器副本留存 _savedClose 供 D6 代际场景二次驱动；
//   - 构造计数：模块级 constructed 计数器，构造器内递增——「零新连接/立即 attempt」
//     断言材料（各场景以基线快照取相对值，跨场景单调不累读）；
//   - beforeunload 记账：包装 window.addEventListener/removeEventListener，对
//     'beforeunload' 类型分别递增 on/off（D1 移除后重注册 / D6 stale 不拆除断言材料；
//     包装转发原实现，行为零漂移）；
//   - attach fetch 闸（loadTerminal opts.holdAttachFetchN）：hold 第 N 次 fetch 的
//     resolve（请求照常发出、服务端真实应答，仅 promise 放行被闸，ctx.releaseHeldFetch
//     放行）——D10 双在飞 attempt 较旧链迟到成功的复现材料。
//
// 隔离纪律：每场景独立 spawn 实例 + 独立 jsdom——重连状态是页面级，隔离防串扰。
//
// 红线（phase04.mjs:6-9 纪律沿用）：token/凭据值永不进 check detail/控制台输出/汇总行——
// detail 只打状态码/布尔/形状/文案常量；assertOutputClean() 运行时自净断言兜底（review #7）。
//
// 运行：node web/uat/phase06-dom.mjs [wesh 二进制路径]（默认 /tmp/wesh-uat/wesh）
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

// SpyWebSocket 构造计数（模块级，构造器内递增）——D2/D3/D4/D5/D8 的「零新连接 /
// 立即 attempt」断言材料；各场景以基线快照取相对值
let constructed = 0;
// startWesh 解析 stdout 时把分享链接 token 留入本闭包数组（只作 assertOutputClean
// 断言材料）——红线：token 值永不进 check detail/控制台输出/汇总行
const sensitiveTokens = [];

// 分享链接 URL → token（/s/{token}/ 路径段；值只作 assertOutputClean 断言材料——红线）
const tokenFromUrl = (url) => /\/s\/([^/]+)\//.exec(url)[1];

// 启动 wesh 实例（phase05.mjs startWesh 同形态：--bind 127.0.0.1 --port 0 + stdout
// 解析 + stderr 捕获 + child 句柄 + SIGKILL kill）。本脚本场景 plain '/' 进入
// （无认证模式，/api/attach 404 探测直连链路既有），分享链接行不用于导航。
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

// 加载 dist 到 jsdom 并执行 bundle（phase05-dom.mjs loadTerminal 形态逐字复用 +
// SpyWebSocket 合成 close / 构造计数 / beforeunload 记账三延伸）。全场景无认证形态
// （plain '/' 进入；服务端无认证模式 /api/attach 404 → 前端跳过 ticket 直连 WS）。
async function loadTerminal(srv, opts = {}) {
  const html = readFileSync(DIST, 'utf8');
  const js = /<script type="module" crossorigin>([\s\S]*?)<\/script>/.exec(html)[1];
  const origin = `${srv.scheme}://${srv.host ?? '127.0.0.1'}:${srv.port}`;
  const url = `${origin}${opts.path ?? '/'}`;
  const dom = new JSDOM('', { url, pretendToBeVisual: true, runScripts: 'outside-only' });
  const { window } = dom;

  // ── 平台能力注入/桩 ──
  // SpyWebSocket：记录全部上行帧首字节 + 合成 CloseEvent 能力 + 实例台账
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
  window.WebSocket = class extends WebSocket {
    constructor(...a) {
      super(...a);
      constructed++; // 模块级构造计数——「零新连接/立即 attempt」断言材料
      this.binaryType = 'arraybuffer';
      this._savedClose = null; // synthClose 留存的 onclose 处理器副本（D6 二次驱动用）
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
  };
  // CR-01 D10 夹具：hold 第 holdN 次 fetch 的 resolve（双在飞 attempt 较旧链迟到成功
  // 复现——请求照常发出、服务端真实应答，仅 resolve 被闸至 releaseHeldFetch 放行；
  // 与「网络闪断后 TCP 重传迟到成功」同形态）。本应用 fetch 唯一用途即 /api/attach
  //（main.ts 单调用点），按调用序号计数安全
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
  // beforeunload 记账：包装转发原实现（行为零漂移），on/off 计数供 D1/D6 断言
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

  // D6 代际守卫场景二次驱动：直接调用 synthClose 留存的处理器副本（review D6 简化
  // 建议形态——synthClose 后 this.onclose 已置 null，代际事件须以捕获副本驱动）
  const staleClose = (inst, code) => {
    inst._savedClose?.call(inst, makeCloseEvent(code));
  };

  return { window, document: window.document, sentFrames, infos, warns, unhandled, dom, dims, sockets, bu, staleClose, releaseHeldFetch: () => releaseHeld?.() };
}

// 等终端完成握手（WELCOME 处理完）：shell prompt 非空白字符出现即 OUTPUT 流通
// （phase05-dom.mjs waitReady 同形态——必须 trim，空行占位不 trim 则抢跑 WELCOME）
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

// 终端 DOM 可见文本（清屏断言通道——term.clear() 后 buffer 行重渲染，异步由 waitFor 吸纳）
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

// ═══════════════════ D1：1006 重连全链（面板 → 退避自动重连 → 清屏 → 重注册） ═══════════════════
async function d1FullReconnectChain() {
  console.log('D1: 1006 重连全链（Reconnecting 面板 C-9 → 退避自动重连 → term.clear() 清屏 → beforeunload 重注册）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const base = constructed;
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
  try {
    await waitReady(ctx.document);
    check('D1a', '会话建立：面板隐藏 + 构造计数 +1',
      !panel(ctx.document).visible && constructed === base + 1,
      `构造=${constructed - base} 面板可见=${panel(ctx.document).visible}`);
    check('D1b', 'WELCOME 后 beforeunload 已注册（记账 on=1/off=0）',
      ctx.bu.on === 1 && ctx.bu.off === 0, `on=${ctx.bu.on} off=${ctx.bu.off}`);
    // 清屏对照文本经 echo 写入（must_have「终端经 echo 写入可观测文本」落此——真实
    // 键盘链路 typeText → INPUT → PTY → echo 输出 → OUTPUT → 终端 DOM；先断言 BANNER
    // 存在再驱动断连，「消失」断言因前置存在性而有意义，review 吸收形态）
    typeText(ctx.window, 'echo D1BANNER\n');
    await waitFor(() => terminalText(ctx.document).includes('D1BANNER'), 'D1BANNER 经 echo 写入终端 DOM');
    check('D1c', 'echo 标记串写入终端 DOM（清屏对照基线在场）', true, 'D1BANNER 在场');

    // 合成 1006 异常断开（RESEARCH A2 兑现）
    ctx.sockets[ctx.sockets.length - 1].synthClose(1006);

    // C-9 逐字要点：title=='Reconnecting'、body 含 attempt 1、hint 双要点
    const p1 = await waitFor(() => {
      const p = panel(ctx.document);
      return p.visible && p.title === 'Reconnecting' ? p : null;
    }, 'Reconnecting 面板出现');
    check('D1d', "1006 → Reconnecting 面板（C-9：title + body 含 'attempt 1' + hint 含 'Reconnect now' 与 'restart it from your shell'）",
      p1.body.includes('attempt 1')
      && p1.hint.includes('Reconnect now')
      && p1.hint.includes('restart it from your shell'),
      `body=${JSON.stringify(p1.body)}`);
    check('D1e', '断开即移除 beforeunload 监听（off +1）',
      ctx.bu.off === 1, `off=${ctx.bu.off}`);

    // 标称退避 1s 后自动重连——waitFor 上限 3s 容差窗（轮询+容差形态替代精确时点断言，review #6）
    await waitFor(() => constructed === base + 2, '退避后自动重连（构造计数 +1）', 3000);
    check('D1f', '退避到期自动发起新连接（SpyWebSocket 构造计数 +1）', true, `构造=${constructed - base}`);

    // 新 WELCOME 到达 = 重连成功唯一判定：面板隐藏 + term.clear() 可观测
    await waitFor(() => !panel(ctx.document).visible, '新 WELCOME 后面板隐藏', 3000);
    check('D1g', '新 WELCOME 到达后面板隐藏（重连成功，D-05）', true, '面板已隐藏');
    await waitFor(() => !terminalText(ctx.document).includes('D1BANNER'), 'D1BANNER 从终端 DOM 消失', 3000);
    check('D1h', '断开前写入的终端文本在重连后从 DOM 消失（term.clear() 可观测证据）',
      true, 'D1BANNER 已消失');
    check('D1i', '重连 WELCOME 后 beforeunload 重注册（on +1，off 不增）',
      ctx.bu.on === 2 && ctx.bu.off === 1, `on=${ctx.bu.on} off=${ctx.bu.off}`);
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D2：1002 不触发边界（C-5 手动面板 + 守候窗零构造） ═══════════════════
async function d2ProtocolErrorNoReconnect() {
  console.log('D2: 1002 不触发边界（Connection lost 手动面板 + 2.5s 守候窗零新连接）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const base = constructed;
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
  try {
    await waitReady(ctx.document);
    ctx.sockets[ctx.sockets.length - 1].synthClose(1002);
    const p = await waitFor(() => {
      const q = panel(ctx.document);
      return q.visible && q.title === 'Connection lost' ? q : null;
    }, 'Connection lost 面板（C-5）');
    check('D2a', '1002 → C-5 Connection lost 手动面板（default 桶残留语义不变）',
      p.body === 'The connection closed unexpectedly.', `body=${JSON.stringify(p.body)}`);
    // 2.5s 守候窗：> attempt 1 退避标称 1s + 1.5s 容差吸收调度抖动（review #6 容差论证）——
    // 窗内零构造 = 不触发的行为证据（若误入重连循环，首个 attempt ~1s 退避点必构造新连接）
    const atPanel = constructed;
    await sleep(2500);
    check('D2b', '2.5s 守候窗内零新连接构造（1002 协议错误不卷入重试循环）',
      constructed === atPanel, `守候窗构造增量=${constructed - atPanel}`);
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D3：1013/1008 不触发边界（专版面板 + 守候窗零构造） ═══════════════════
async function d3KickedAndRefusedNoReconnect() {
  console.log('D3: 1013/1008 不触发边界（Disconnected / Connection refused 专版 + 2.5s 守候窗零新连接）');
  // ① 1013 慢消费者被踢 → 维持手动刷新（P5 D-10，D-01 确认保持）
  const inst1 = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const ctx1 = await loadTerminal({ scheme: inst1.scheme, port: inst1.port });
  try {
    await waitReady(ctx1.document);
    ctx1.sockets[ctx1.sockets.length - 1].synthClose(1013);
    const p = await waitFor(() => {
      const q = panel(ctx1.document);
      return q.visible && q.title === 'Disconnected' ? q : null;
    }, 'Disconnected 面板（C-1）');
    check('D3a', '1013 → Disconnected 专版（C-1 文案，手动刷新语义保持）',
      /could not keep up with the session output/.test(p.body), `body=${JSON.stringify(p.body)}`);
    const atPanel1 = constructed;
    await sleep(2500); // 守候窗容差论证同 D2b
    check('D3b', '2.5s 守候窗内零新连接构造（1013 被踢不自动重连）',
      constructed === atPanel1, `守候窗构造增量=${constructed - atPanel1}`);
  } finally {
    await cleanup(ctx1, inst1);
  }
  // ② 1008 抽样代表 1000/1008/1009/1011 专版集合（同集合逐字面板由 main.ts 既有分支
  // 承载——1000 由 D7 真实 EXIT 全链覆盖，1009/1011 与 1008 同 hint 同分派形态，抽样一档）
  const inst2 = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const ctx2 = await loadTerminal({ scheme: inst2.scheme, port: inst2.port });
  try {
    await waitReady(ctx2.document);
    ctx2.sockets[ctx2.sockets.length - 1].synthClose(1008);
    const p = await waitFor(() => {
      const q = panel(ctx2.document);
      return q.visible && q.title === 'Connection refused' ? q : null;
    }, 'Connection refused 面板');
    check('D3c', '1008 → Connection refused 专版（1000/1008/1009/1011 集合抽样）',
      p.body === 'The server refused this connection.', `body=${JSON.stringify(p.body)}`);
    const atPanel2 = constructed;
    await sleep(2500); // 守候窗容差论证同 D2b
    check('D3d', '2.5s 守候窗内零新连接构造（1008 不自动重连）',
      constructed === atPanel2, `守候窗构造增量=${constructed - atPanel2}`);
  } finally {
    await cleanup(ctx2, inst2);
  }
}

// ═══════════════════ D4：双触发幂等（offline + onclose(1006) 相继到达 → 单循环） ═══════════════════
async function d4DoubleTriggerIdempotent() {
  console.log('D4: 双触发幂等（RESEARCH Pitfall 5：断网瞬间双触发不得启双循环）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const base = constructed;
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
  try {
    await waitReady(ctx.document);
    // 断网瞬间两触发源相继到达：onclose(1006) 先启循环，offline 紧随——reconnecting
    // 门闩幂等返回（Pitfall 5 防线；offline+onclose 各启一循环是回归形态）
    ctx.sockets[ctx.sockets.length - 1].synthClose(1006);
    ctx.window.dispatchEvent(new ctx.window.Event('offline'));
    const p = await waitFor(() => {
      const q = panel(ctx.document);
      return q.visible && q.title === 'Reconnecting' ? q : null;
    }, 'Reconnecting 面板出现');
    check('D4a', '双触发后单循环启动（Reconnecting 面板 attempt 计数不跳变）',
      p.body.includes('attempt 1'), `body=${JSON.stringify(p.body)}`);
    // 标称退避 1s 后恰好一个新连接（构造计数按退避节奏单调 +1——双循环会并发多构造）
    await waitFor(() => constructed === base + 2, '退避后自动重连（构造计数 +1）', 3000);
    await waitFor(() => !panel(ctx.document).visible, '重连成功面板隐藏', 3000);
    check('D4b', '自动重连成功（构造计数 +1 且面板隐藏）', true, `构造=${constructed - base}`);
    // 3s 守候窗（覆盖 attempt 2 的 2s 退避点）：计数不并发翻倍 = 无第二循环残留
    await sleep(3000);
    check('D4c', '3s 窗内构造计数不并发翻倍（双触发未启双循环）',
      constructed === base + 2, `构造=${constructed - base}`);
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D5：Reconnect now 手动入口（点击跳过等待，循环不终止） ═══════════════════
async function d5ReconnectNowManual() {
  console.log('D5: Reconnect now 手动入口（等待期点击 hint 链接 → 倒计时未完即发起新连接）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const base = constructed;
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
  try {
    await waitReady(ctx.document);
    ctx.sockets[ctx.sockets.length - 1].synthClose(1006);
    await waitFor(() => {
      const q = panel(ctx.document);
      return q.visible && q.title === 'Reconnecting' ? q : null;
    }, 'Reconnecting 面板出现（等待期）');
    const link = ctx.document.querySelector('#status-hint a');
    check('D5a', "等待期 hint 内 'Reconnect now' 链接在场（#status-hint a）",
      link?.textContent === 'Reconnect now', `link=${JSON.stringify(link?.textContent ?? '')}`);
    link.click();
    // 800ms 容差窗 ≪ 标称退避 1s（轮询+容差形态，review #6 吸收）：窗内到达 = 等待被
    // 跳过（退避到期前 +1）；超时未到 = 未跳过 = FAIL
    await waitFor(() => constructed === base + 2, '点击后立即 attempt（构造计数 +1）', 800);
    check('D5b', '点击后 800ms 容差窗内发起新连接（倒计时未完即 attempt）', true, `构造=${constructed - base}`);
    // 循环不终止于手动入口——若本次失败仍按退避继续；本场景成功以 WELCOME 终止
    await waitFor(() => !panel(ctx.document).visible, '新 WELCOME 后面板隐藏', 3000);
    check('D5c', '手动入口后循环以成功终止（新 WELCOME 面板隐藏）', true, '面板已隐藏');
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D6：代际守卫（旧 socket 迟到 onclose 零污染新会话） ═══════════════════
async function d6StaleGenerationGuard() {
  console.log('D6: 代际守卫（RESEARCH Pitfall 6：stale onclose 不得触碰新会话状态）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const base = constructed;
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
  try {
    await waitReady(ctx.document);
    const stale = ctx.sockets[ctx.sockets.length - 1]; // 首连接实例引用留存
    stale.synthClose(1006);
    await waitFor(() => constructed === base + 2, '退避后自动重连', 3000);
    await waitFor(() => !panel(ctx.document).visible, '重连成功面板隐藏', 3000);
    // 重连成功新会话建立后，对旧实例迟到派发合成 onclose(1006)——四 handler 入口
    // if (sock !== ws) return 代际守卫的直击断言（Pitfall 6 防线）
    ctx.staleClose(stale, 1006);
    await sleep(400); // 吸纳窗：守卫失效的污染同步显形（面板/新连接/拆监听）
    check('D6a', 'stale onclose 后面板保持隐藏（新会话状态零污染）',
      !panel(ctx.document).visible, `面板可见=${panel(ctx.document).visible}`);
    check('D6b', 'stale onclose 后零新连接构造（无第三连接）',
      constructed === base + 2, `构造=${constructed - base}`);
    check('D6c', 'stale onclose 不拆新连接 beforeunload 监听（off 不增）',
      ctx.bu.off === 1 && ctx.bu.on === 2, `on=${ctx.bu.on} off=${ctx.bu.off}`);
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D7：EXIT 帧全链（真实服务端 exit 7 → Session ended 逐字） ═══════════════════
async function d7ExitFrameChain() {
  console.log('D7: EXIT 帧全链（真实服务端行为：子进程 exit 7 → EXIT 帧 + 1000 → Session ended 正文逐字）');
  const inst = await startWesh(['--', 'sh', '-c', 'sleep 2; exit 7']);
  const base = constructed;
  // wesh 进程退出码捕获（exitf 退出码传递顺带锁定）——提早挂监听防错过
  const exitCodeP = new Promise((r) => inst.child.once('exit', (code) => r(code)));
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
  try {
    // 会话建立（2s 窗口内）——sh 无输出，以 WELCOME 处理完成的 beforeunload 注册为可观测代理
    await waitFor(() => ctx.bu.on === 1, '会话建立（WELCOME 处理完成，beforeunload 注册）', 2000);
    // 子进程到期退出 → 真实 EXIT 帧 + 1000 到达 → Session ended 面板
    const p = await waitFor(() => {
      const q = panel(ctx.document);
      return q.visible && q.title === 'Session ended' ? q : null;
    }, 'Session ended 面板', 8000);
    check('D7a', "Session ended 正文逐字 == 'The process exited with code 7.'（06-01 服务端组文案前端直显端到端证据）",
      p.body === 'The process exited with code 7.', `body=${JSON.stringify(p.body)}`);
    const exitCode = await exitCodeP;
    check('D7b', 'wesh 进程 exit 事件码==7（进程级退出码传递）',
      exitCode === 7, `exit=${exitCode}`);
    check('D7c', '1000 明确终结不触发重连（零新连接构造）',
      constructed === base + 1, `构造=${constructed - base}`);
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D8：online 快路径（清等待立即 attempt，D-04） ═══════════════════
async function d8OnlineFastPath() {
  console.log('D8: online 快路径（重连等待期 dispatch online → 当前等待被清立即 attempt）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const base = constructed;
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port });
  try {
    await waitReady(ctx.document);
    ctx.sockets[ctx.sockets.length - 1].synthClose(1006);
    await waitFor(() => {
      const q = panel(ctx.document);
      return q.visible && q.title === 'Reconnecting' ? q : null;
    }, 'Reconnecting 面板出现（等待期，未到 1s 退避点）');
    // online = 清当前等待定时器立即试一次——不是新循环（D-04）
    ctx.window.dispatchEvent(new ctx.window.Event('online'));
    // 800ms 容差窗 ≪ 标称退避 1s（轮询+容差形态同 D5，review #6）
    await waitFor(() => constructed === base + 2, 'online 后立即 attempt（构造计数 +1）', 800);
    check('D8a', 'online dispatch 后 800ms 容差窗内发起新连接（退避到期前 +1）', true, `构造=${constructed - base}`);
    await waitFor(() => !panel(ctx.document).visible, '新 WELCOME 后面板隐藏', 3000);
    check('D8b', 'online 快路径重连成功（面板隐藏）', true, '面板已隐藏');
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D10：CR-01 代际守卫 fetch 半侧（stale 迟到成功不踩占健康连接） ═══════════════════
async function d10StaleLateSuccessNoClobber() {
  console.log('D10: CR-01 双在飞 attempt 迟到成功不踩占（stale fetch 迟到 resolve → 零新构造/面板隐藏/无 ro 降级）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  const base = constructed;
  // hold 第 2 次 fetch = 重连 attempt 1 的 attach 请求（第 1 次为首连，正常放行）
  const ctx = await loadTerminal({ scheme: inst.scheme, port: inst.port }, { holdAttachFetchN: 2 });
  try {
    await waitReady(ctx.document);
    // 1006 → 重连循环 → 退避 1s 到期 attempt 1：connect() 同步发出 fetch #2（被闸悬挂）——
    // runAttempt 在途正文出现即 fetch 已发出（runAttempt → void connect() 同步段单任务内完成）
    ctx.sockets[ctx.sockets.length - 1].synthClose(1006);
    await waitFor(() => panel(ctx.document).body.includes('Retrying now'), 'attempt 1 在途（fetch #2 已发出被闸）', 3000);
    // attempt 1 的 fetch 悬挂期间点击「Reconnect now」→ attempt 2（D-04 既定双在飞形态）：
    // fetch #3 正常放行 → 新连接 → WELCOME → 健康会话建立在 attempt 2 上
    ctx.document.querySelector('#status-hint a').click();
    await waitFor(() => constructed === base + 2, 'attempt 2 建连（构造计数 +1）', 3000);
    await waitFor(() => !panel(ctx.document).visible, 'attempt 2 WELCOME 后面板隐藏', 3000);
    // 放行被闸的 stale fetch #2（迟到成功）：修复前 404 探测后无条件 ws = new WebSocket
    // 踩占句柄（构造 +1，stale socket attach 成 ro 写 [ro] 前缀 + disableStdin）；
    // 修复后 gen 代际复查①直接丢弃返回
    ctx.releaseHeldFetch();
    await sleep(400); // 吸纳窗（D6 同款）：若守卫失效，踩占连接同步构造并 attach 显形
    check('D10a', 'stale fetch 迟到 resolve 后零新连接构造（未踩占健康句柄）',
      constructed === base + 2, `构造=${constructed - base}`);
    check('D10b', '面板保持隐藏（健康会话 UI 零污染）',
      !panel(ctx.document).visible, `面板可见=${panel(ctx.document).visible}`);
    check('D10c', "标题无 '[ro] ' 前缀（未被踩占连接的 ro attach 降级——修复前会写 [ro] + disableStdin）",
      !ctx.document.title.startsWith('[ro] '), `title=${JSON.stringify(ctx.document.title)}`);
  } finally {
    await cleanup(ctx, inst);
  }
}

// ═══════════════════ D9：真实断网栈豁免（headless 硬约束，人工清单指针） ═══════════════════
function d9RealNetworkStackExempt() {
  skip('D9', '真实 OS 断网栈与浏览器原生 online/offline 事件时序',
    'headless 硬约束豁免（CODEBUDDY.md 平台原生行为豁免条款）：本脚本以合成 CloseEvent/Event dispatch 驱动同一状态机覆盖等价逻辑面，06-06 phase06.mjs 以真实 TCP 断连覆盖协议面；真实栈场景人工清单见 .planning/phases/06-session-lifecycle/06-UAT.md（06-07 产出，06-UI-SPEC §UI Considerations backstop 行）');
}

// 输出自净断言（review #7 吸收——红线由注释纪律升级为运行时自证）：遍历全部已发
// detail，断言不含任一 token 值或 '/s/' 链接形态串；命中即 FAIL（防未来回归静默破线，
// phase04.mjs:6-9 红线的可执行形态）。命中时不回显冒犯内容（只打布尔/计数——红线自保）
function assertOutputClean() {
  const leaked = emittedDetails.some((d) =>
    d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t)));
  check('SEC', "输出自净：全部 detail 零 token 值零 '/s/' 链接形态串（红线运行时自证）",
    !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}

const scenarios = [d1FullReconnectChain, d2ProtocolErrorNoReconnect, d3KickedAndRefusedNoReconnect, d4DoubleTriggerIdempotent, d5ReconnectNowManual, d6StaleGenerationGuard, d7ExitFrameChain, d8OnlineFastPath, d10StaleLateSuccessNoClobber, d9RealNetworkStackExempt];
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
