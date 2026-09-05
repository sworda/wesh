// Phase 12 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch/child_process/net）。
// 覆盖 PC-05/PC-06/PC-07/PC-10/PC-11 协议面——六场景一次建齐（12-CONTEXT D-13：
// Go 测试 12-01/12-02/12-03 证明内部不变量，本脚本证明真实二进制在真实协议面上
// 的端到端行为；jsdom 前端断言面归 phase12-dom.mjs，Playwright 归 Phase 14）：
//   S1 Welcome.session 双模式对照（per-client 实例收 "per-client"、shared 实例收
//      "shared"，且 mode/cols/rows 既有键同帧共存——D-08 协议面端到端证据）；
//   S2 resize 直通双端隔离（A RESIZE(120,50) → A 的 stty size 变 "50 120"、B 仍
//      为 Hello 尺寸，且双端 attach Welcome 后零 'W' 帧——PC-05 SC1 协议层证据）；
//   S3 ro RESIZE 直通（per-client ro 端 RESIZE 后经 SIGWINCH trap 回读新尺寸到达
//      OUTPUT；shared 同形态对照零新输出——D-06/D-07 wire 面证据）；
//   S4 ro INPUT 丢弃（零回显）+ rw 限速保留（flood 后不踢不断、后续输入照常被
//      处理——PC-07 协议层证据）；
//   S5 停读期输出不丢、恢复后完整到达（stall 客户端 TCP 停读 → seq 洪水积压 →
//      恢复读 → 序号连续无缺口，且对端全程不受影响——PC-11 端到端证据）；
//   S6 真实 10s+ dwell 到期 → 1013 slow_consumer 端到端（D-12——真实等待，时钟
//      不 mock、服务端 dwell 不缩短；phase06 保活测 11s+ 先例）。
//
// 红线（phase11.mjs:27-30 纪律逐字沿用）：token/凭据/pid 数值只作断言材料，永不
// 进入 check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/退出码/
// 文案常量（pid 断言一律以布尔「不等/ESRCH 到达」表达；测试输出可能进 CI 日志，
// 敏感值落盘即泄露样本）。
//
// 时序纪律（phase06.mjs:354-356 容差论证先例经 phase11.mjs:32-33 继承）：宽限/
// 退避/免疫/dwell 类场景真实等待，超时上限只做护栏，禁精确时点断言；时钟不 mock、
// 服务端等待不缩短（dwell 走生产默认 10s，零覆写零测试钩子——D-12）。
//
// 运行：node web/uat/phase12.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh；
// 先构建：go build -o /tmp/wesh-uat/wesh ./cmd/wesh）
import { spawn } from 'node:child_process';
import net from 'node:net';
import crypto from 'node:crypto';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐——D-16 两侧注释互指纪律）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
// Welcome 载荷 JSON {"mode","session","cols","rows","prefs"?}：12-01 起恒携 session
// 模式位键（proto.go WelcomePayload D-08，"shared"|"per-client" 与 CLI flag
// --session-mode 同词同值域；恒序列化无 omitempty——旧服务端缺席该键 = shared）。
const SUBPROTOCOL = 'wesh.v1';

const enc = new TextEncoder();
const dec = new TextDecoder();
const concat = (...parts) => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) { out.set(p, off); off += p.length; }
  return out;
};
// Hello 载荷 {version,cols,rows}（无认证实例不携 ticket——checkTicket 对 nil
// store 既有跳过形态，omitempty 对称：无认证模式前端不出 ticket 键）
const helloFrame = ({ cols = 80, rows = 24 } = {}) =>
  concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols, rows })));

const results = [];
// 全部已发 detail 收集（assertOutputClean 遍历材料——红线运行时自净断言）
const emittedDetails = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  emittedDetails.push(String(detail));
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
// 平台豁免记录形态：不计失败（CODEBUDDY.md 分层测试策略 §5 显式豁免条款）
const skip = (id, name, reason) => {
  results.push({ id, name, ok: null });
  emittedDetails.push(String(reason));
  console.log(`  SKIP  ${id} ${name} — ${reason}`);
};

// startWesh 解析 stdout 时把分享链接 token 留入本闭包数组（只作 assertOutputClean
// 断言材料）——红线：token 值永不进 check detail/控制台输出/汇总行
const sensitiveTokens = [];
// 会话 pid 数值同红线处理（解析结果只作断言材料——pid 断言一律以布尔
// 「不等/ESRCH 到达」表达，数值永不进 detail）
const sensitivePids = [];

// 分享链接 URL → token（/s/{token}/ 路径段；值只作断言材料——红线）
const tokenFromUrl = (url) => /\/s\/([^/]+)\//.exec(url)[1];

// 启动超时 reject 消息脱敏（WR-02 先例沿用——异常消息进控制台/CI 日志，argv
// 原样回显会把凭据值明文送进日志；本脚本场景不带凭据，防御形态保留）
const redactArgs = (args) => args.map((a, i) => {
  if (a.startsWith('--credential=')) return '--credential=<redacted>';
  if (i > 0 && args[i - 1] === '--credential') return '<redacted>';
  return a;
}).join(' ');

// 启动 wesh 实例（spawn 真实二进制），解析实际端口与分享链接两行，返回
// { port, shareRO, shareRW, stderrText, kill, child }。显式 --bind 127.0.0.1 +
// --port 0（loopback 随机端口，与用户服务零干扰——phase02.mjs D-03 形态）。
// stdout 两行解析：listening on 行 + share read-only: 行（TCP 形态恒打印）；ro 行
// 齐备后 50ms 落定窗吸纳 rw 行可能的管道分块边界；stderr 持续捕获（logEvent/panic
// 断言通道）。子进程追踪收口：kill 恒 SIGKILL（CPU 受限 CI 泄漏级联减速实证纪律）。
function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    let stdoutBuf = '';
    let settling = false;
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${redactArgs(args)}; stderr=${stderr}`)); }, 8000);
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

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// 建立 WS 连接并完成 Hello 握手（可定尺寸）；返回 { ws, frames }，frames 持续累积。
// Welcome 到达即视为握手完成；10s watchdog 防挂死（IN-04 先例——被测二进制挂死时
// 拒绝而非永久悬挂）。
function dialHello(port, { cols = 80, rows = 24 } = {}) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame({ cols, rows }));
    ws.onerror = () => reject(new Error('WS 连接失败'));
    const watchdog = setTimeout(() => {
      clearInterval(poll);
      reject(new Error('握手总超时：10s 未收到 Welcome'));
    }, 10000);
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) { clearInterval(poll); clearTimeout(watchdog); resolve({ ws, frames }); }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); clearTimeout(watchdog); reject(new Error(`握手被关闭 code=${ev.code} reason=${ev.reason}`)); };
  });
}

const waitClose = (ws, timeoutMs) => new Promise((resolve) => {
  const to = setTimeout(() => resolve(null), timeoutMs);
  ws.onclose = (ev) => { clearTimeout(to); resolve({ code: ev.code, reason: ev.reason }); };
});

// sendInput：INPUT 类型字节 + 载荷组帧发送
const sendInput = (ws, text) => ws.send(concat(new Uint8Array([INPUT]), enc.encode(text)));
// sendResize：RESIZE 类型字节 + {"cols":C,"rows":R} JSON 组帧（proto.DecodeResize
// 钳制 [1,1000] 在解码层既有——直通路径消费同一 Decode，D-16）
const sendResize = (ws, cols, rows) =>
  ws.send(concat(new Uint8Array([RESIZE]), enc.encode(JSON.stringify({ cols, rows }))));

const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');

// 取 frames 中首帧 WELCOME 的 JSON 载荷（Welcome 恒首帧不变量——S5 时序纪律）
const welcomeOf = (frames) => {
  const f = frames.find((x) => x[0] === WELCOME);
  return f ? JSON.parse(dec.decode(f.subarray(1))) : null;
};

// echoMark：唯一标记回读（会话存活/健康探针——INPUT echo <mark>，OUTPUT 含标记）
const echoMark = async (frames, ws, mark, timeoutMs = 5000) => {
  const base = frames.length;
  sendInput(ws, `echo ${mark}\r`);
  const t0 = Date.now();
  while (Date.now() - t0 < timeoutMs) {
    if (outputText(frames, base).includes(mark)) return true;
    await sleep(50);
  }
  return false;
};

// pgroupAlive：进程组存活探针（setsid 不变量使 pgid==pid，echo 回读的 pid 即 pgid
// 锚点——phase11.mjs 形态）。红线：无错返回 = 存活探针，严禁当死亡证据；EPERM 等
// 意外形态上抛（同用户下不可达）——fail-closed。
const pgroupAlive = (pid) => {
  try { process.kill(-pid, 0); return true; } catch (e) {
    if (e.code === 'ESRCH') return false;
    throw e;
  }
};

// pollESRCH：每 50ms process.kill(-pid, 0) 探测进程组，ESRCH 即到达（僵尸未收割
// 则组仍存在——ESRCH ⊇ 收割完成强证据）。总护栏参数化，超时返回 false（由断言
// 转 FAIL，非无限等待——T-06-06b 护栏先例）。
const pollESRCH = async (pid, guardMs) => {
  const t0 = Date.now();
  while (Date.now() - t0 < guardMs) {
    if (!pgroupAlive(pid)) return true;
    await sleep(50);
  }
  return false;
};

// healthzClients：GET /healthz 的 clients 字段值（registry.n 计数源）。停读类
// 场景的踢出观测通道（12-03 纪律）：只读 HTTP 面不打扰 WS stall 面——读取 WS
// 即破坏停读，轮询替代固定 sleep（STATE Phase 9 教训的通道面应用）。
const healthzClients = async (port) => {
  const r = await fetch(`http://127.0.0.1:${port}/healthz`);
  const j = await r.json();
  return j.clients;
};

// RawStallClient：raw net.Socket 手工 WS 客户端（phase05.mjs rawStallClient
// 纪律的一般化）。停读必须用 raw socket 而非 Node WebSocket 客户端——undici
// 实现会持续 drain TCP，内核级停读结构性不可达（phase05.mjs:162-163 纪律登记）；
// 停读/续读以 socket.pause()/resume() 表达（slowclient_test.go「dialHello 后不再
// Read」的 Node 语义对应）。C→S 帧 masked（RFC6455）；S→C 帧不 masked；writer
// 的同类型连续段合并使单条 WS 消息可达 outbox cap（512KiB）——解析器支持 16/64
// 位扩展长度。ping 自动回 pong（与浏览器协议栈自动应答语义一致——连接存活不因
// 停读观测通道而异）。OUTPUT 载荷拷贝脱离解析视图后累积（P5-1 同族纪律：跨
// chunk 持有须拷贝）。
class RawStallClient {
  constructor() {
    this.frames = [];      // 非 OUTPUT 应用帧载荷（W/E——数量恒小）
    this.outChunks = [];   // OUTPUT 载荷字节累积
    this.outBytes = 0;
    this.tail = '';        // 输出尾窗滚动 ~8KiB（marker/pid 检索面）
    this.closeInfo = null; // close 帧 {code, reason}
    this.tcpClosed = false;
    this.pending = null;   // 跨 chunk 半帧残余
    this.welcomeDone = false;
  }

  connect(port, { cols = 80, rows = 24 } = {}) {
    return new Promise((resolve, reject) => {
      const key = crypto.randomBytes(16).toString('base64');
      const socket = net.connect(port, '127.0.0.1');
      this.socket = socket;
      let hsBuf = Buffer.alloc(0);
      let upgraded = false;
      const watchdog = setTimeout(() => { socket.destroy(); reject(new Error('raw 握手总超时：10s 未收到 Welcome')); }, 10000);
      socket.on('error', (e) => { this.errorInfo = e; clearTimeout(watchdog); reject(e); });
      socket.on('close', () => { this.tcpClosed = true; });
      socket.on('data', (d) => {
        if (!upgraded) {
          hsBuf = Buffer.concat([hsBuf, d]);
          const idx = hsBuf.indexOf('\r\n\r\n');
          if (idx === -1) return;
          const head = hsBuf.subarray(0, idx).toString('latin1');
          if (!head.includes(' 101')) { socket.destroy(); clearTimeout(watchdog); reject(new Error(`WS 升级非 101: ${head.split('\r\n')[0]}`)); return; }
          upgraded = true;
          // masked Hello 完成注册（服务端 Welcome 到达 = 握手完成，dialHello 同判据）
          this.send(helloFrame({ cols, rows }));
          const rest = hsBuf.subarray(idx + 4);
          if (rest.length > 0) this.feed(rest);
          return;
        }
        this.feed(d);
        if (this.welcomeDone) { clearTimeout(watchdog); resolve(this); }
      });
      socket.on('connect', () => {
        socket.write(
          `GET /ws HTTP/1.1\r\nHost: 127.0.0.1:${port}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n` +
          `Sec-WebSocket-Key: ${key}\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Protocol: ${SUBPROTOCOL}\r\n\r\n`);
      });
    });
  }

  // send：masked binary 数据帧（本脚本全部 C→S 帧 <126B——Hello/INPUT 均短帧形）
  send(payload) {
    if (payload.length > 125) throw new Error(`raw client 短帧上限超限: ${payload.length}`);
    this.sendMasked(0x2, payload);
  }

  sendMasked(opcode, payload) {
    const mask = crypto.randomBytes(4);
    const masked = Buffer.allocUnsafe(payload.length);
    for (let i = 0; i < payload.length; i++) masked[i] = payload[i] ^ mask[i & 3];
    this.socket.write(Buffer.concat([Buffer.from([0x80 | opcode, 0x80 | payload.length]), mask, masked]));
  }

  pause() { this.socket.pause(); }  // 停读起点——内核接收缓冲停止排空
  resume() { this.socket.resume(); } // 续读——缓冲数据经 data 事件流入解析器

  destroy() { this.socket.destroy(); }

  // feed：RFC6455 帧解析（服务端帧不 masked；长度 7/16/64 位三形）
  feed(chunk) {
    const buf = this.pending ? Buffer.concat([this.pending, chunk]) : chunk;
    this.pending = null;
    let pos = 0;
    for (;;) {
      const avail = buf.length - pos;
      if (avail < 2) break;
      const opcode = buf[pos] & 0x0f;
      let len = buf[pos + 1] & 0x7f;
      let off = pos + 2;
      if (len === 126) {
        if (avail < 4) break;
        len = buf.readUInt16BE(off); off += 2;
      } else if (len === 127) {
        if (avail < 10) break;
        len = buf.readUInt32BE(off) * 2 ** 32 + buf.readUInt32BE(off + 4); off += 8;
      }
      if (avail < off - pos + len) break;
      this.handleFrame(opcode, buf.subarray(off, off + len));
      pos = off + len;
    }
    this.pending = pos === 0 ? buf : pos === buf.length ? null : Buffer.from(buf.subarray(pos));
  }

  handleFrame(opcode, payload) {
    if (opcode === 0x1 || opcode === 0x2) { // 应用帧（服务端恒 binary）
      if (payload.length === 0) return;
      const t = payload[0]; // wesh 帧类型字节
      if (t === OUTPUT) {
        const p = Buffer.from(payload.subarray(1)); // 拷贝脱离解析视图
        this.outChunks.push(p);
        this.outBytes += p.length;
        this.tail = (this.tail + dec.decode(p)).slice(-8192);
      } else {
        this.frames.push(Buffer.from(payload));
        if (t === WELCOME) this.welcomeDone = true;
      }
      return;
    }
    if (opcode === 0x9) { this.sendMasked(0xA, payload); return; } // ping→pong
    if (opcode === 0xA) return; // pong——无需消费
    if (opcode === 0x8) { // close：记录 {code,reason} 后回 echo close 帧（服务端
      // Close 等待对端关闭帧收口，5s 超时——礼貌回帧使握手即刻落定）
      this.closeInfo = {
        code: payload.length >= 2 ? payload.readUInt16BE(0) : 1005,
        reason: dec.decode(payload.subarray(2)),
      };
      try { this.sendMasked(0x8, payload); } catch { /* socket 已终结 */ }
      this.socket.end();
    }
  }

  // readPid：发 `echo <TAG>=$$` 回读会话 pid（phase11 readPid 的 raw 半侧——正则
  // 只命中结果行；pid 数值只作断言材料，红线：永不进 detail）
  async readPid(tag, timeoutMs = 5000) {
    this.send(concat(new Uint8Array([INPUT]), enc.encode(`echo ${tag}=$$\r`)));
    const t0 = Date.now();
    while (Date.now() - t0 < timeoutMs) {
      const m = new RegExp(`${tag}=(\\d+)`).exec(this.tail);
      if (m) return Number(m[1]);
      await sleep(50);
    }
    return null;
  }
}

// seqContinuity：洪水字节流连续性算术步进校验（Go assertSeqContinuity 同语义的
// 字节级强化——fields 严格 +1 无缺口、首值恒 1、末位精确；PTY ONLCR 使行尾
// CRLF）。锚定洪水首 10 行联合形态（回显命令行/提示符不可能包含）后逐行精确
// 匹配，任何缺段/损坏即在断裂处报因——停读/续读转换丢段 = PC-11 违反。
function seqContinuity(buf, last) {
  const head = [];
  for (let i = 1; i <= 10; i++) head.push(String(i));
  const anchor = Buffer.from(head.join('\r\n'));
  const start = buf.indexOf(anchor);
  if (start === -1) return { ok: false, why: '洪水起点锚未命中' };
  let i = start;
  let n = 1;
  while (n <= last) {
    const s = String(n);
    for (let k = 0; k < s.length; k++) {
      if (buf[i++] !== s.charCodeAt(k)) return { ok: false, why: `第 ${n} 行数字不匹配（字节流损坏或丢段）` };
    }
    if (buf[i++] !== 0x0d || buf[i++] !== 0x0a) return { ok: false, why: `第 ${n} 行行尾非 CRLF` };
    n++;
  }
  return { ok: true, bytes: i - start };
}

// ---------- S1：Welcome.session 双模式对照（PC-06/D-08 协议面端到端） ----------
async function s1WelcomeSession() {
  console.log('S1: Welcome.session 双模式对照（per-client 实例=="per-client" / shared 实例=="shared"，mode/cols/rows 同帧共存）');
  const pc = await startWesh(['--session-mode=per-client', '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(pc.port, {});
    const w = welcomeOf(c.frames);
    check('S1a', 'per-client 实例 Welcome：session=="per-client" 且 mode/cols/rows 既有键同帧在场（D-08 additive 不挤压）',
      w?.session === 'per-client' && w?.mode === 'rw' && w?.cols === 80 && w?.rows === 24,
      `session=${w?.session} mode=${w?.mode} cols=${w?.cols} rows=${w?.rows}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    pc.kill();
  }
  const sh = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(sh.port, {});
    const w = welcomeOf(c.frames);
    check('S1b', 'shared 实例（默认模式）Welcome：session=="shared" 且 mode/cols/rows 既有键同帧在场',
      w?.session === 'shared' && w?.mode === 'rw' && w?.cols === 80 && w?.rows === 24,
      `session=${w?.session} mode=${w?.mode} cols=${w?.cols} rows=${w?.rows}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    sh.kill();
  }
}

// ---------- S2：resize 直通双端隔离 + 零 'W' 帧（PC-05 SC1 协议层证据） ----------
async function s2ResizePassthroughIsolation() {
  console.log('S2: resize 直通双端隔离（A RESIZE(120,50)→stty "50 120" / B 仍 Hello 尺寸 "28 90"；双端 attach Welcome 后零 W 帧）');
  const inst = await startWesh(['--session-mode=per-client', '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const a = await dialHello(inst.port, { cols: 100, rows: 30 });
    const b = await dialHello(inst.port, { cols: 90, rows: 28 });
    // 排空两端在途帧（提示符/回显）——「零 W」断言与 stty 基线的排空纪律
    //（11-04 drainQuiet 形态的 UAT 同构）
    const drainedA = await echoMark(a.frames, a.ws, 'UAT_S2_DA_p3k8');
    const drainedB = await echoMark(b.frames, b.ws, 'UAT_S2_DB_m6v2');
    await sleep(300); // 两端落定窗（提示符尾帧吸纳）
    // A RESIZE(120,50)——per-client 直通本会话 TIOCSWINSZ（50ms 防抖保留）
    sendResize(a.ws, 120, 50);
    await sleep(400); // 防抖 50ms × 8 落定窗（护栏性等待，非精确时点断言）
    // A 端回读新尺寸（stty size 输出 rows cols 序："50 120"；回显 "stty size"
    // 无数字不干扰——phase11 S2b 结果行命中纪律）
    const baseA = a.frames.length;
    sendInput(a.ws, 'stty size\r');
    let aNew = false;
    const tA = Date.now();
    while (Date.now() - tA < 5000 && !aNew) {
      aNew = outputText(a.frames, baseA).includes('50 120');
      if (!aNew) await sleep(50);
    }
    check('S2a', 'A RESIZE(120,50) 后 A 端 stty size 回读 "50 120"（直通本会话 PTY，尺寸生效）',
      drainedA && aNew, `回读命中=${aNew}`);
    // B 端回读仍为其 Hello 尺寸（90x28 → "28 90"）——A 的 RESIZE 对 B 零影响
    const baseB = b.frames.length;
    sendInput(b.ws, 'stty size\r');
    let bSame = false;
    const tB = Date.now();
    while (Date.now() - tB < 5000 && !bSame) {
      bSame = outputText(b.frames, baseB).includes('28 90');
      if (!bSame) await sleep(50);
    }
    check('S2b', 'B 端 stty size 仍回读 "28 90"（自身 Hello 尺寸——双端隔离零影响）',
      drainedB && bSame, `回读命中=${bSame}`);
    // 静默窗后断言双端 attach Welcome（首帧）之外零 'W' 帧——per-client 无仲裁器、
    // 无运行期尺寸再推送（12-02 直通分支零 recalcNow/pushSessionDimsLocked）
    await sleep(1200);
    const wExtraA = a.frames.slice(1).filter((f) => f[0] === WELCOME).length;
    const wExtraB = b.frames.slice(1).filter((f) => f[0] === WELCOME).length;
    check('S2c', '双端 attach Welcome 后零 "W" 帧到达（无仲裁器运行期约束/尺寸推送——PC-05 SC1）',
      wExtraA === 0 && wExtraB === 0,
      `A端额外W=${wExtraA} B端额外W=${wExtraB}`);
    a.ws.close(1000); b.ws.close(1000);
    await waitClose(a.ws, 3000); await waitClose(b.ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S3：ro RESIZE 直通（D-06/D-07 wire 面）+ shared 对照（D-09 第二闸逐字保留） ----------
// trap 脚本：启动先打一次基线 stty size；WINCH trap 打当前尺寸；短 sleep 主循环——
// POSIX trap 在前台命令完成后执行，sleep 10 会把 trap 延迟到命令边界（≤0.2s 循环
// 使 trap 时延 ≤~250ms + 防抖 50ms，全部落在 5s 护栏内）。argv 经 spawn 数组传递
// 不经 shell 拼接（exec 数组纪律，P1 既有）；bash -c 非交互不读 rc 文件。
const WINCH_TRAP_SCRIPT = "stty size; trap 'stty size' WINCH; while true; do sleep 0.2; done";
async function s3ROResizePassthrough() {
  console.log('S3: ro RESIZE 直通（per-client ro 端 RESIZE(133,55)→SIGWINCH trap 回读 "55 133"；shared 同操作对照零新输出）');
  const pc = await startWesh(['--session-mode=per-client', '--', 'bash', '-c', WINCH_TRAP_SCRIPT]);
  try {
    const c = await dialHello(pc.port, {});
    const w = welcomeOf(c.frames);
    // 基线：bash 启动即打印 Hello 钳制尺寸（80x24 → "24 80"）
    let baseline = false;
    const t0 = Date.now();
    while (Date.now() - t0 < 5000 && !baseline) {
      baseline = outputText(c.frames).includes('24 80');
      if (!baseline) await sleep(50);
    }
    check('S3a', '前置：per-client ro 会话（无 --writable）基线尺寸行 "24 80" 到达（mode=ro）',
      w?.mode === 'ro' && baseline, `mode=${w?.mode} 基线命中=${baseline}`);
    const base = c.frames.length;
    // ro 端 RESIZE——D-06：per-client 第二闸不生效，ro 直通自己 PTY（ttyd parity）
    sendResize(c.ws, 133, 55);
    let hit = false;
    const t1 = Date.now();
    while (Date.now() - t1 < 5000 && !hit) {
      hit = outputText(c.frames, base).includes('55 133');
      if (!hit) await sleep(50);
    }
    check('S3b', 'per-client ro 端 RESIZE(133,55) → trap 回读新尺寸行 "55 133" 到达 OUTPUT（直通→TIOCSWINSZ→SIGWINCH→trap，D-06）',
      hit, `新尺寸行命中=${hit}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    pc.kill();
  }
  // shared 对照（D-09 第二闸：server.go ro RESIZE continue 丢弃——shared 逐字保留）
  const sh = await startWesh(['--', 'bash', '-c', WINCH_TRAP_SCRIPT]);
  try {
    const c = await dialHello(sh.port, {});
    const w = welcomeOf(c.frames);
    let baseline = false;
    const t0 = Date.now();
    while (Date.now() - t0 < 5000 && !baseline) {
      baseline = outputText(c.frames).includes('24 80');
      if (!baseline) await sleep(50);
    }
    check('S3c', '前置：shared ro 会话基线尺寸行 "24 80" 到达（对照半场就位）',
      w?.mode === 'ro' && baseline, `mode=${w?.mode} 基线命中=${baseline}`);
    const base = c.frames.length;
    sendResize(c.ws, 133, 55);
    await sleep(1500); // 静默窗（防抖 50ms × 30 + trap 时延上界；护栏性等待）
    const out = outputText(c.frames, base);
    check('S3d', 'shared ro 端同操作对照：静默窗零新输出零 "55 133"（D-09 第二闸 shared 逐字保留）',
      out.length === 0 && !out.includes('55 133'),
      `窗内新输出字节=${out.length} 新尺寸行=${out.includes('55 133')}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    sh.kill();
  }
}

// ---------- S4：ro INPUT 丢弃（零回显）+ rw 限速保留（PC-07 协议层证据） ----------
async function s4ROInputDropRateLimit() {
  console.log('S4: ro INPUT 丢弃（零回显）+ rw 限速保留（超速率 flood 不踢不断、后续输入照常处理）');
  // 半场一：per-client 无 --writable → ro 客户端，INPUT 服务端丢弃（12-02
  // TestPerClientROInputDropped 的 UAT 同构——丢弃是 mode 闸语义而非链路故障）
  const ro = await startWesh(['--session-mode=per-client', '--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(ro.port, {});
    const w = welcomeOf(c.frames);
    check('S4a', '前置：per-client 无 --writable → Welcome mode=="ro"',
      w?.mode === 'ro', `mode=${w?.mode}`);
    let closed = false;
    c.ws.onclose = () => { closed = true; };
    await sleep(500); // 提示符落定窗（bash 启动提示符到达后才记静默基线，防晚到假阳）
    const base = c.frames.length;
    sendInput(c.ws, 'echo UAT_S4_RO_MARK_q7w1\r');
    await sleep(1200); // 静默窗——ro INPUT 被服务端丢弃：零回显（INPUT 不过 mode 闸，PTY 零感知）
    const out = outputText(c.frames, base);
    check('S4b', 'ro 端 INPUT "echo MARKER" → 静默窗零回显零输出零断开（服务端丢弃，连接不受影响）',
      !out.includes('UAT_S4_RO_MARK_q7w1') && !closed,
      `标记零命中=${!out.includes('UAT_S4_RO_MARK_q7w1')} 连接存活=${!closed} 窗内字节=${out.length}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    ro.kill();
  }
  // 半场二：per-client rw 限速保留（defaultInputRate 32KiB/s + burst 64KiB——超限
  // 唯一动作 = 丢弃该帧（RES-02 drop 语义），禁止以限速为由 Close/踢出）
  const rw = await startWesh(['--session-mode=per-client', '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(rw.port, {});
    let closed = false;
    c.ws.onclose = () => { closed = true; };
    // 超速率洪水：120 帧 × 15KiB（单帧 < 16KiB ReadLimitPostAuth 硬顶），20ms 间隔
    // 持续 ~2.4s 共 ~1.8MB ≫ 令牌桶通过量（burst 64KiB + 32KiB/s × 2.4s ≈ 141KiB）
    // ——大部分帧被限速丢弃。载荷 '#' 注释字符：通过限速的帧经 canonical 行缓冲
    // 由 "\r" 收口后成注释行，shell 静默消化（12-02 探针防误丢两步纪律）。
    const flood = '#'.repeat(15 * 1024);
    for (let i = 0; i < 120 && !closed; i++) {
      sendInput(c.ws, flood);
      await sleep(20);
    }
    await sleep(400); // 令牌回充窗（32KiB/s × 0.4s = 12.8KiB ≫ 探针帧 ~20B）
    sendInput(c.ws, '\r'); // canonical 行缓冲收口——洪水残余 '#' 行以注释执行
    sendInput(c.ws, 'echo UAT_S4_RW_MARK_z9d4\r');
    const base = c.frames.length;
    let marker = false;
    const t0 = Date.now();
    while (Date.now() - t0 < 5000 && !marker) {
      marker = outputText(c.frames, base).includes('UAT_S4_RW_MARK_z9d4');
      if (!marker) await sleep(50);
    }
    check('S4c', 'rw 端超速率 flood（120×15KiB / ~2.4s ≫ 32KiB/s）后连接存活 + echo 探针正常回显（丢弃语义不踢不断）',
      marker && !closed,
      `探针回读=${marker} 连接存活=${!closed}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    rw.kill();
  }
}

// ---------- S5：停读期输出不丢、恢复后完整到达（PC-11 端到端证据） ----------
// 洪水量 seq 1 4000000 ≈ 30.9MB（slowclient_test.go seqFlood Linux 分支同款）：
// loopback 单连接最坏吸收 ≈ wmem 4MiB + rmem 6MiB + outbox 512KiB + PTY 64KiB
// ≈ 10.6MiB（slowclient_test.go:8-11 纪律），30.9MB ≈ 3× 余量使停读态确定形成
// （TCP 物理上限迫使 writer 阻塞 → outbox 涨满 → 闭包阻塞持帧 + dwell 武装——
// 停读形成由管线量级保证，协议层无服务端计数可读，行为证据 = 恢复后字节连续）。
// 时序：停读窗 ~3s ≪ dwell 10s（安全边 3×+）；恢复后不发任何 INPUT 直至洪水
// 收齐——tty 回显与洪水输出共用 PTY 输出流，恢复期发标记会中途插入回显字节
// 破坏连续性校验面（收齐信号 = 尾窗含 "3999999\r\n4000000\r\n" 终态联合形态，
// 回显行不可能包含——结果行锚定纪律）。
async function s5StallResumeNoLoss() {
  console.log('S5: 停读续读不丢（A raw socket 停读 → seq 洪水积压 → 3s 停读窗 ≪ dwell 10s → 恢复读 → 序号连续无缺口；B 全程不受影响）');
  const FLOOD_LAST = 4000000;
  // WR-01（12-REVIEW）：与 S6 同款 --ping-interval=0 隔离——S6 注释实证的竞态
  //（默认 ping 5s 下，stall 端 writer 阻塞持 writeFrameMu 时 ping tick 的
  // writeControl 内层 5s 写超时返回 DeadlineExceeded，被 pinger 误读为 pong
  // 超时 → 1006 先杀）同样作用于 S5 的 RawStallClient（pause 停读同时停了自动
  // pong、writer 同样会 mu 阻塞）：停读窗标称 ~3s，慢 CI 上 B 探针（echoMark
  // 5s 超时上界）+ 30.9MB 管线排空延迟可把 writer mu 阻塞窗拉过首个 ping tick
  //（attach+5s），tick+5s 处触发 1006 → S5a/S5b 假阳 FAIL。S5 断言面（序号
  // 连续/连接存活/对端无扰）零依赖保活，隔离零弱化
  const inst = await startWesh(['--session-mode=per-client', '--writable', '--ping-interval=0', '--', 'sh']);
  const a = new RawStallClient();
  try {
    const b = await dialHello(inst.port, {});
    await a.connect(inst.port, {});
    // 洪水命令发出后立即停读（INPUT 写出不受 pause 影响——pause 只停读侧）
    a.send(concat(new Uint8Array([INPUT]), enc.encode(`seq 1 ${FLOOD_LAST}\r`)));
    a.pause(); // 停读起点——dwell 自 outbox 涨满点武装（管线填满 ~1s ≪ 停读窗）
    // B 端停读窗内探针（per-client 双端独立会话——A 停读对 B 结构性零影响）
    const bDuring = await echoMark(b.frames, b.ws, 'UAT_S5_B_DURING_c4j9');
    await sleep(2700); // 补足 ~3s 停读窗（B 探针 ~0.3s + 落定）
    a.resume(); // 续读——积压（内核双缓冲 + outbox + 持帧）全量流入
    // 收齐信号轮询（护栏 30s：慢 CI 全速排空 ~20MB 的上界；本机常态 <2s）
    let drained = false;
    const t0 = Date.now();
    while (Date.now() - t0 < 30000 && !drained) {
      drained = a.tail.includes('3999999\r\n4000000\r\n');
      if (!drained) await sleep(100);
    }
    // S5a：序号连续性（字节级算术步进——停读期积压 + 持帧 + 恢复续读全链零丢失）
    const cont = drained ? seqContinuity(Buffer.concat(a.outChunks), FLOOD_LAST) : { ok: false, why: '洪水未收齐' };
    check('S5a', '恢复读后 seq 1..4000000 序号严格 +1 连续无缺口（停读期输出不丢，PC-11）',
      cont.ok, cont.ok ? `校验字节=${cont.bytes}` : `断裂: ${cont.why}`);
    // S5b：连接存活无 1013（停读窗 3s ≪ dwell 10s 未被踢）+ 会话照常——洪水收齐后
    // 才发 POST 探针（此时回显字节落在已校验洪水之后，零污染）
    let post = false;
    if (drained) {
      a.send(concat(new Uint8Array([INPUT]), enc.encode('echo UAT_S5_POST_t8n3\r')));
      const t1 = Date.now();
      while (Date.now() - t1 < 5000 && !post) {
        post = /\nUAT_S5_POST_t8n3/.test(a.tail); // 结果行锚定（排除命令回显行）
        if (!post) await sleep(50);
      }
    }
    check('S5b', 'A 连接存活无 1013（close 未发生）+ 洪水收齐后 echo 探针正常回显（会话照常）',
      a.closeInfo === null && !a.tcpClosed && post,
      `零close=${a.closeInfo === null} TCP存活=${!a.tcpClosed} 探针=${post}`);
    // S5c：B 端全程不受影响（停读窗内 + 收齐后 echo 照常）
    const bAfter = await echoMark(b.frames, b.ws, 'UAT_S5_B_AFTER_h2r6');
    check('S5c', 'B 端停读窗内 + 窗后 echo 照常（对端全程不受影响）',
      bDuring && bAfter, `窗内=${bDuring} 窗后=${bAfter}`);
    b.ws.close(1000);
    await waitClose(b.ws, 3000);
  } finally {
    a.destroy();
    inst.kill();
  }
}

// ---------- S6：真实 10s+ dwell 到期 → 1013 slow_consumer 端到端（D-12/PC-10） ----------
// dwell 走生产默认 defaultSlowDwell=10s，零覆写零测试钩子（D-12 硬约束：时钟不
// mock、服务端等待不缩短——Go 侧 500ms 覆写测已锁定机制，本场景做一次真实 10s+
// 等待的端到端证据，phase06 保活测 11s+ 先例）。观测通道：/healthz clients 归零
// 轮询（只读 HTTP 不打扰 WS stall 面，12-03 纪律）检测踢出后恢复读——被踢端
// writer 阻塞写在满 TCP 上持 writeFrameMu，恢复读使阻塞写完成、kick 路径的
// writeClose 获锁补发 1013 关闭帧（clients.go:625-632 关闭帧可达性不变量）。
// --writable：pid 回读与洪水命令经 INPUT 通道（ro 实例 INPUT 被丢弃无法注洪水；
// dwell 踢出与客户端 mode 无关——per-client 停读闭包模式无关）。
//
// --ping-interval=0 的裁决依据（执行期实证发现，Rule 3 场景形态修正）：coder/
// websocket writeControl 内层 5s 写超时（write.go:277-279）——默认 --ping-
// interval=5s 下，ping tick 落在 writer 持锁阻塞于满 TCP 的窗口时，mu.lock 以
// ctx 超时等待 5s 后返回 DeadlineExceeded，被 pinger 误读为 pong 超时（server.go
// pinger 只认 errors.Is(err, DeadlineExceeded) 单一形态）→ TCP 级停读客户端在
// (停读点+5s, 停读点+10s] 被 1006 pong_timeout 先杀，dwell 1013 结构性后到
// （实测 detach 恰于 attach+10.0007s = tick+5.0007s，reason=pong_timeout）。
// Go 侧 12-03 四测未暴露此交互——harness Options.PingInterval 零值即 pinger
// 禁用。本场景以生产 CLI flag --ping-interval=0（D-16「0 = 禁用保活」公开契约）
// 隔离 dwell 看门狗做端到端证据；默认 ping 配置下 1006/1013 竞态的取舍归
// Phase 13 裁决（STATE Blockers 登记——真实浏览器端网络栈自动回 pong 不触发
// 该路径，herdr 类自管 socket 客户端可触发）。
async function s6DwellKick1013() {
  console.log('S6: 真实 dwell 到期 1013（单端停读恒不读 → 生产 10s dwell 到期 → /healthz 归零 → 恢复读收 1013 close → ESRCH 收割复核）');
  const FLOOD_LAST = 4000000;
  const inst = await startWesh(['--session-mode=per-client', '--writable', '--ping-interval=0', '--', 'sh']);
  const a = new RawStallClient();
  let pid = null;
  try {
    await a.connect(inst.port, {});
    pid = await a.readPid('S6PID');
    if (pid !== null) sensitivePids.push(pid);
    const clients1 = await healthzClients(inst.port);
    check('S6a', '前置：A attach + pid 回读 + /healthz clients==1（停读前就位）',
      pid !== null && clients1 === 1, `pid解析=${pid !== null} clients==1=${clients1 === 1}`);
    if (pid !== null) {
      // 停读：洪水 + pause 后恒不读（dwell 自 outbox 涨满点武装）
      a.send(concat(new Uint8Array([INPUT]), enc.encode(`seq 1 ${FLOOD_LAST}\r`)));
      a.pause();
      const stallStart = Date.now();
      // 踢出检测：/healthz clients 1→0（护栏 25s = dwell 10s + 管线填满/调度余量）
      let kicked = false;
      while (Date.now() - stallStart < 25000) {
        if ((await healthzClients(inst.port)) === 0) { kicked = true; break; }
        await sleep(150);
      }
      const dwellWaitMs = Date.now() - stallStart;
      check('S6b', '停读恒不读 → 真实 dwell（生产 10s 零覆写）到期踢出：/healthz clients 归零（护栏 25s 内）',
        kicked && dwellWaitMs >= 10000,
        `踢出=${kicked} 实测停读至踢出=${(dwellWaitMs / 1000).toFixed(1)}s（≥10s 生产值）`);
      // 恢复读收 1013（护栏 15s：恢复后管线排空 + writeClose 补发的上界）
      a.resume();
      const t0 = Date.now();
      while (Date.now() - t0 < 15000 && a.closeInfo === null && !a.tcpClosed) await sleep(50);
      check('S6c', '恢复读后 CloseError code==1013 且 reason=="slow_consumer"（R-10 机器串逐字）',
        a.closeInfo?.code === 1013 && a.closeInfo?.reason === 'slow_consumer',
        `close=${a.closeInfo?.code ?? '未到达'} reason逐字=${a.closeInfo?.reason === 'slow_consumer'}`);
      // 踢出 → teardown SIGHUP → 进程组收割（PC-03 挂点联动证据，phase11 S4 形态）
      const gone = await pollESRCH(pid, 3000);
      check('S6d', '踢出后 3s 护栏内进程组 ESRCH（断开即杀——PC-03 挂点联动）',
        gone, `ESRCH到达=${gone}`);
    } else {
      check('S6b', '真实 dwell 到期 1013', false, '前提失败：pid 未解析');
      check('S6c', 'CloseError 1013 slow_consumer', false, '前提失败：pid 未解析');
      check('S6d', '进程组 ESRCH', false, '前提失败：pid 未解析');
    }
  } finally {
    // CI 夹具纪律：断言失败路径也不泄漏滞留进程组（ESRCH 幂等静默）
    if (pid !== null) { try { process.kill(-pid, 'SIGKILL'); } catch { /* 已消亡 */ } }
    a.destroy();
    inst.kill();
  }
}


// 输出自净断言（phase11.mjs:563-573 逐字——红线由注释纪律升级为运行时自证）：
// 遍历全部已发 detail，断言不含任一 share token 值（含 '/s/' 链接形态串）与任一
// 会话 pid 数值；命中即 FAIL（防未来回归静默破线）。命中时不回显冒犯内容
// （只打布尔/计数——红线自保）。
function assertOutputClean() {
  const leaked = emittedDetails.some((d) =>
    d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t))
    || sensitivePids.some((p) => d.includes(String(p))));
  check('SEC', "输出自净：全部 detail 零 token 值零 pid 数值零 '/s/' 链接形态串（红线运行时自证）",
    !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}

// 六场景串行收口——场景间 300ms + 异常纳入 emittedDetails + skipped 不阻塞
// 退出码（phase11.mjs:576-595 逐字；与头注释六场景清单逐一对账）
const scenarios = [s1WelcomeSession, s2ResizePassthroughIsolation, s3ROResizePassthrough, s4ROInputDropRateLimit, s5StallResumeNoLoss, s6DwellKick1013];
let failed = 0;
for (const s of scenarios) {
  try {
    await s();
  } catch (e) {
    failed++;
    // 异常消息纳入 emittedDetails——assertOutputClean 自净断言面延伸到场景异常
    // 通道（WR-02 先例；startWesh 启动超时等消息可携敏感值静默破线）
    emittedDetails.push(String(e.message));
    console.log(`  FAIL  场景异常: ${e.message}`);
  }
  await sleep(300);
}
assertOutputClean();
const skipped = results.filter((r) => r.ok === null).length;
const passedN = results.filter((r) => r.ok === true).length;
const failedN = results.filter((r) => r.ok === false).length;
console.log(`\n结果: ${passedN}/${results.length - skipped} 协议断言通过${skipped ? `，${skipped} 项 skipped（豁免）` : ''}${failed ? `，${failed} 个场景异常` : ''}`);
process.exit(failedN === 0 && failed === 0 ? 0 : 1);
