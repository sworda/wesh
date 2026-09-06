// Phase 13 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch/child_process）。
// 覆盖 PC-08/PC-09/SEC-09/OPS-12 协议面——六场景一次建齐（13-07：Go 测试
// 13-01..13-06 证明内部不变量，本脚本证明真实二进制 + 真实 wire + 真实信号下
// 防线与终结语义的端到端行为；phase12.mjs 同构第三代，jsdom/Playwright 归
// Phase 14）：
//   S1 churn 节流 + XFF 换键（无认证实例 loopback 单 IP 高频 attach 超 per-IP
//      burst 4 → 某端收 Error{server_error}+close 1011 逐值 + stderr
//      spawn_throttled 事件 + throttled 计数器；--auth-header 开启后各异
//      XFF 独立桶互不挤占（多 IP 形态）/ 同值连发超 burst 被节流且事件
//      remote==XFF 链首（单 IP 形态）——D-04/D-05 双态）；
//   S2 KILL 兜底默认 5s（per-client 零配置 trap '' HUP 免疫 → 断开 ~2s 时点
//      存活 → 5s 到期 SIGKILL 收割 ESRCH；显式 --stop-timeout=0 对照 → 6s 窗
//      后免疫进程仍存活（泄漏形态）+ stderr 泄漏 warn 行 + finally kill -9
//      清理 + pgrep 残留零命中——D-01/D-02）；
//   S3 退出三形态 per-client 进程级 255（--once / --exit-when-empty 裸 flag
//      grace=0 / =2s 宽限到期；宽限内重连 → 取消存活（跨原到期点无 exit）+
//      再断开新宽限到期退出——accept-255 裁决的进程级映射）；
//   S4 Shutdown N 组（双客户端双独立 sh → SIGTERM → 双端 1001
//      server_shutting_down + 双 pgid ESRCH + stderr session_end 事件数==2
//      且 signal==SIGHUP + 服务端进程退出 255）；
//   S5 metrics 四计数器 + session_active 计数语义 + 零身份 label（per-client
//      attach 1 → 四 series 全在 + wesh_pty_spawn_total≥1 +
//      wesh_session_active==1 + HELP per-client 会话计数文案 + 样本行零
//      label 花括号；shared 对照四计数器恒 0 series 保留不摘 + HELP 探活
//      文案逐字——D-07/D-08 + PC-09 prohibition）；
//   S6 WESH_REMOTE_USER env 可见（per-client + --auth-header 携头 attach →
//      子进程 env 回读含 WESH_REMOTE_USER=头值 sanitize 产物（干净值直通 /
//      C1 控制字符注入剥离形态）；shared 对照同头 → env 无该键 + 会话照常
//      ——SEC-09 注入链 / D-15 收窄零漂移）。
//
// 红线（phase12.mjs:18-21 纪律逐字沿用）：token/凭据/pid 数值只作断言材料，永不
// 进入 check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/退出码/
// 文案常量（pid 断言一律以布尔「不等/ESRCH 到达」表达；测试输出可能进 CI 日志，
// 敏感值落盘即泄露样本）。
//
// 时序纪律（phase06.mjs:354-356 容差论证先例经 phase11/12 继承）：宽限/
// 退避/免疫/dwell 类场景真实等待，超时上限只做护栏，禁精确时点断言；时钟不 mock、
// 服务端等待不缩短（S2 KILL 兜底走 per-client 默认 5s 零覆写零测试钩子——D-01
// 「零配置」形态的端到端意义所在；phase06 保活测 11s+ / phase12 dwell 10s+ 同款）。
// --once/--exit-when-empty 场景的服务端进程退出是特性不是回归——child 'exit'
// 事件即断言通道（waitExit，phase06 先例）。
//
// 运行：node web/uat/phase13.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh；
// 先构建：go build -o /tmp/wesh-uat/wesh ./cmd/wesh）
import { spawn, execFileSync } from 'node:child_process';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐——D-16 两侧注释互指纪律；RESIZE 本
// phase 场景不消费，协议表完整保留）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
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
// 会话 pid 数值同红线处理（readPid/waitScanPid 解析结果只作断言材料——pid 断言
// 一律以布尔「不等/ESRCH 到达」表达，数值永不进 detail）
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
// 齐备后 50ms 落定窗吸纳 rw 行可能的管道分块边界；stderr 持续捕获（logEvent/warn
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

// 建立 WS 连接并完成 Hello 握手（可定尺寸、可携反代头）；返回 { ws, frames }，
// frames 持续累积。opts.headers（默认无）：S1/S6 的 XFF / X-Remote-User 头注入
// ——Node >= 22 原生 WebSocket 第二参 { headers, protocols } 形态（phase07.mjs
// 本机探针实证：自定义头与 C1 控制字符均可传输）。Welcome 到达即视为握手完成；
// 10s watchdog 防挂死（IN-04 先例——被测二进制挂死时拒绝而非永久悬挂）。
function dialHello(port, { cols = 80, rows = 24, headers } = {}) {
  return new Promise((resolve, reject) => {
    const url = `ws://127.0.0.1:${port}/ws`;
    const ws = headers === undefined
      ? new WebSocket(url, [SUBPROTOCOL])
      : new WebSocket(url, { headers, protocols: [SUBPROTOCOL] });
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

// dialAttach：churn 循环的统一 dial——Welcome 到达 = attach 成功（resolve
// {ok:true, ws, frames}，调用方立即 close 完成 connect→Hello→close 一循环）；
// close 先于 Welcome 到达 = 拒绝路径（resolve {ok:false, close:{code,reason},
// frames}——Error 帧在 frames 内供逐值断言；dialHello/dialExpectReject 两形态
// 的合流，phase11.mjs T4b 拒绝收集 + phase12.mjs dialHello 握手先例）。10s
// watchdog 防挂死。
function dialAttach(port, { cols = 80, rows = 24, headers } = {}, timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    const url = `ws://127.0.0.1:${port}/ws`;
    const ws = headers === undefined
      ? new WebSocket(url, [SUBPROTOCOL])
      : new WebSocket(url, { headers, protocols: [SUBPROTOCOL] });
    ws.binaryType = 'arraybuffer';
    const frames = [];
    let settled = false;
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame({ cols, rows }));
    ws.onerror = () => { if (!settled) reject(new Error('WS 连接失败')); };
    const watchdog = setTimeout(() => {
      if (settled) return;
      clearInterval(poll);
      reject(new Error('dialAttach 总超时：未收到 Welcome 亦未关闭'));
    }, timeoutMs);
    const poll = setInterval(() => {
      if (settled) { clearInterval(poll); return; }
      if (frames.some((f) => f[0] === WELCOME)) {
        settled = true;
        clearInterval(poll); clearTimeout(watchdog);
        resolve({ ok: true, ws, frames });
      }
    }, 10);
    ws.onclose = (ev) => {
      if (settled) return;
      settled = true;
      clearInterval(poll); clearTimeout(watchdog);
      resolve({ ok: false, close: { code: ev.code, reason: ev.reason }, frames });
    };
  });
}

const waitClose = (ws, timeoutMs) => new Promise((resolve) => {
  const to = setTimeout(() => resolve(null), timeoutMs);
  ws.onclose = (ev) => { clearTimeout(to); resolve({ code: ev.code, reason: ev.reason }); };
});

// waitExit：child 'exit' 事件决议 {code, signal}——S3/S4 的 wesh 进程退出断言
// 通道（本 phase 场景的服务端退出是特性不是回归，phase06 先例）。恒带超时护栏：
// 被测二进制挂死时护栏到期 resolve(null) 由断言转 FAIL，而非无限等待。
function waitExit(child, timeoutMs) {
  return new Promise((resolve) => {
    const to = setTimeout(() => resolve(null), timeoutMs);
    child.once('exit', (code, signal) => { clearTimeout(to); resolve({ code, signal }); });
  });
}

// 帧收集器 collectUntilClose(ws)：换装 onmessage/onclose 为本收集器，close 到达
// 时决议 {frames, close:{code,reason}}（phase11/12 逐字——S4 的 1001 关停序列
// 断言通道）。
function collectUntilClose(ws, timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    const frames = [];
    const to = setTimeout(() => reject(new Error(`collectUntilClose 超时：${timeoutMs}ms 未收到 close`)), timeoutMs);
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onclose = (ev) => { clearTimeout(to); resolve({ frames, close: { code: ev.code, reason: ev.reason } }); };
  });
}

// sendInput：INPUT 类型字节 + 载荷组帧发送
const sendInput = (ws, text) => ws.send(concat(new Uint8Array([INPUT]), enc.encode(text)));

const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');

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

// readPid：发 `echo <TAG>=$$` 回读会话 pid（phase11.mjs 正则数字锚定纪律——回显
// 含命令原文（无数字不命中），正则只命中结果行）。pid 数值只作断言材料（红线：
// 永不进 detail——以布尔「解析成功/不等」表达）。
const readPid = async (frames, ws, tag) => {
  const base = frames.length;
  sendInput(ws, `echo ${tag}=$$\r`);
  const t1 = Date.now();
  while (Date.now() - t1 < 5000) {
    const m = new RegExp(`${tag}=(\\d+)`).exec(outputText(frames, base));
    if (m) return Number(m[1]);
    await sleep(50);
  }
  return null;
};

// waitScanPid：从持续累积的既有帧轮询扫描 <TAG>=<pid>（S2 非交互死循环夹具启动
// 即印 pid、不读 stdin——pid 经初始 OUTPUT 回读，不发 INPUT）。
const waitScanPid = async (frames, tag) => {
  const t1 = Date.now();
  while (Date.now() - t1 < 5000) {
    const m = new RegExp(`${tag}=(\\d+)`).exec(outputText(frames));
    if (m) return Number(m[1]);
    await sleep(50);
  }
  return null;
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

// parseEvents：stderr 混合流按行解析 JSON 事件（08-01 D-13 迁移后事件为 slog
// JSON 单行——phase07.mjs 形态）——滤非 '{' 起始行（启动行/警告行等人文本成员
// 不算事件）；'{' 起始行非法 JSON 即抛错。事件值只作断言材料——detail 只打
// event 名/布尔/计数（红线保持）。
const parseEvents = (text) =>
  text.split('\n').flatMap((line, i) => {
    if (!line.startsWith('{')) return [];
    try {
      return [JSON.parse(line)];
    } catch (e) {
      throw new Error(`事件行非合法 JSON（第 ${i + 1} 行）: ${line.slice(0, 120)}: ${e.message}`);
    }
  });

// metricValue：样本行 = name + 空格 + 数值（name+空格前缀精确名匹配不撞前缀族，
// phase08.mjs metricSample 同构）；label 形态行（build_info）不适用。
const metricValue = (body, name) => {
  for (const line of body.split('\n')) {
    if (line.startsWith(name + ' ')) {
      const v = Number(line.slice(name.length).trim());
      return Number.isFinite(v) ? v : null;
    }
  }
  return null;
};

// ---------- S1：churn 节流 + XFF 换键（PC-08/D-04/D-05 协议面端到端） ----------
// 半场一（无认证实例——loopback 单 IP 键）：高频 attach 循环（真实 WebSocket
// connect→Hello→close 一循环/次）。per-IP 桶 1/s burst 4（13-02 生产默认）使
// 首 4 次 attach 放行耗尽 burst、后续在补给窗（1/s）内被结构性拒绝——拒绝序列
// 与容量拒绝同码同串（Error{server_error, "server is at capacity"} + 1011，D-04
// wire 聚合第三次应用），分辨率全在事件名（spawn_throttled vs max_clients——
// 事件面断言二者分治，杜绝把容量拒绝误判为节流的假阳）。
// 半场二（--auth-header 开启——trust on）：XFF 换键双态（D-05）——各异 XFF
// 交替（每桶 3 ≤ burst 4）全部成功（独立桶互不挤占）；同值 XFF 连发超 burst
// 被节流且 spawn_throttled 事件 remote==XFF 链首（换键直接证据）。
async function s1ChurnThrottleXff() {
  console.log('S1: churn 节流 + XFF 换键（A 单 IP 高频 attach 超 burst → 1011 逐值 + spawn_throttled 事件/计数器；B 异 XFF 独立桶不挤占 / 同值超 burst 被节流）');
  const instA = await startWesh(['--session-mode=per-client', '--writable', '--', 'sh']);
  try {
    let okN = 0, rejN = 0, firstRej = null, all1011 = true;
    for (let i = 0; i < 14; i++) {
      const r = await dialAttach(instA.port);
      if (r.ok) { okN++; r.ws.close(1000); await waitClose(r.ws, 2000); }
      else { rejN++; if (firstRej === null) firstRej = r; if (r.close.code !== 1011) all1011 = false; }
    }
    check('S1a', 'churn 循环：成功 ≥4（per-IP burst 4 放行）且拒绝 ≥2（超 burst 后节流）且全部拒绝 close==1011',
      okN >= 4 && rejN >= 2 && all1011,
      `成功=${okN} 拒绝=${rejN} 全1011=${all1011}`);
    // 首个拒绝端逐值断言：恰一 Error{server_error, 容量文案逐字} + close 1011
    const errs = firstRej ? firstRej.frames.filter((f) => f[0] === ERROR) : [];
    const ep = errs.length > 0 ? JSON.parse(dec.decode(errs[0].subarray(1))) : null;
    check('S1b', '首个拒绝端逐值：恰一 Error{server_error, "server is at capacity" 逐字} + close 1011（D-04 wire 聚合）',
      firstRej !== null && errs.length === 1 && ep?.code === 'server_error'
        && ep?.message === 'server is at capacity' && firstRej.close.code === 1011,
      `Error帧数=${errs.length} code=${ep?.code} 文案逐字=${ep?.message === 'server is at capacity'} close=${firstRej?.close?.code}`);
    await sleep(500); // 事件落定窗（stderr 管道异步送达）
    const evs = parseEvents(instA.stderrText());
    const throttled = evs.filter((m) => m.event === 'spawn_throttled');
    const maxClients = evs.filter((m) => m.event === 'max_clients');
    const attachN = evs.filter((m) => m.event === 'attach').length;
    // /metrics 黑盒 scrape：throttled 计数器面证据（must_haves truth 3 的协议层
    // 承载——10rps 级注入结构性触发节流）
    const mresp = await fetch(`http://127.0.0.1:${instA.port}/metrics`);
    const thrTotal = metricValue(await mresp.text(), 'wesh_pty_spawn_throttled_total');
    check('S1c', '事件与计数器面：stderr spawn_throttled==拒绝数 且 /metrics wesh_pty_spawn_throttled_total==拒绝数 且 max_clients==0（事件名分治——拒绝源是节流非容量）且 attach==成功数',
      throttled.length === rejN && thrTotal === rejN && maxClients.length === 0 && attachN === okN,
      `事件=${throttled.length} 计数器=${thrTotal} 拒绝=${rejN} max_clients=${maxClients.length} attach=${attachN}`);
  } finally {
    instA.kill();
  }
  const instB = await startWesh(['--session-mode=per-client', '--writable', '--auth-header', 'X-Remote-User', '--', 'sh']);
  try {
    // 多 IP 形态：双 XFF 交替各 3 次（每桶 3 ≤ burst 4；全局桶 6 ≤ burst 16）→ 全部成功
    let multiOk = 0;
    for (let i = 0; i < 6; i++) {
      const xff = i % 2 === 0 ? '203.0.113.7' : '198.51.100.9';
      const r = await dialAttach(instB.port, { headers: { 'X-Forwarded-For': xff } });
      if (r.ok) { multiOk++; r.ws.close(1000); await waitClose(r.ws, 2000); }
    }
    check('S1d', '多 IP 形态：双 XFF 交替 6 次 attach 全成功（各异 XFF 独立桶互不挤占——D-05 换键）',
      multiOk === 6, `成功=${multiOk}/6`);
    // 单 IP 形态：同值 XFF 连发 6 次 → 首 4 次（burst）放行后第 5 次起 1011
    let singleOk = 0, singleRej = null;
    for (let i = 0; i < 6; i++) {
      const r = await dialAttach(instB.port, { headers: { 'X-Forwarded-For': '192.0.2.50' } });
      if (r.ok) { singleOk++; r.ws.close(1000); await waitClose(r.ws, 2000); }
      else { singleRej = r; break; }
    }
    const errs2 = singleRej ? singleRej.frames.filter((f) => f[0] === ERROR) : [];
    const ep2 = errs2.length > 0 ? JSON.parse(dec.decode(errs2[0].subarray(1))) : null;
    check('S1e', '单 IP 形态：同值 XFF 连发——≥4 成功（burst 放行）后首拒绝 Error{server_error, 容量文案逐字}+close 1011 逐值',
      singleOk >= 4 && singleRej !== null && errs2.length === 1 && ep2?.code === 'server_error'
        && ep2?.message === 'server is at capacity' && singleRej.close.code === 1011,
      `成功=${singleOk} 拒绝到达=${singleRej !== null} 文案逐字=${ep2?.message === 'server is at capacity'} close=${singleRej?.close?.code}`);
    await sleep(500);
    const throttledB = parseEvents(instB.stderrText()).filter((m) => m.event === 'spawn_throttled');
    const remoteAllXff = throttledB.length > 0 && throttledB.every((m) => m.remote === '192.0.2.50');
    check('S1f', 'spawn_throttled 事件 remote==XFF 值（换键直接证据——事件行桶键即 XFF 链首，D-05）',
      throttledB.length >= 1 && remoteAllXff,
      `事件数=${throttledB.length} remote全为XFF=${remoteAllXff}`);
  } finally {
    instB.kill();
  }
}

// ---------- S2：KILL 兜底默认 5s（D-01 零配置生效）与显式 0 对照（D-02） ----------
// 半场一：per-client 零配置（不传 --stop-timeout）——13-01 D-01 默认 5s 使 HUP
// 免疫泄漏防线默认开启。时序双断言（phase11 S8 形态）：断开后 ~2s 时点进程组
// 仍存活（HUP 被 trap 免疫 + KILL 未到期）；随后 12s 护栏内 ESRCH（5s 到期
// SIGKILL 补发收割），且断开至收割 elapsed ≥ 4s（锚定 5s 量级——更早的收割
// 意味着默认值漂移，1s 级错误默认在该下界翻车）。
// 半场二：显式 --stop-timeout=0——D-02 显式位尊重（无 KILL 兜底，泄漏形态是
// 用户明示意图）+ 13-01 validateStartup 泄漏风险 warn 行；断开 6s（> 5s 标称
// + 1s 护栏）后免疫进程仍存活（若显式 0 被误改回默认 5s，此处已被 KILL——
// 判别面）。finally kill -9 清理免疫进程 + 场景尾 pgrep 残留零命中断言
// （must_haves——UAT 自身零泄漏，T-13-23）。
async function s2KillBackstopDefault5s() {
  console.log("S2: KILL 兜底默认 5s（零配置 trap '' HUP → ~2s 存活 → 5s 到期 KILL 收割；显式 --stop-timeout=0 对照泄漏 + warn 行 + kill -9 清理）");
  const inst = await startWesh(['--session-mode=per-client', '--', 'sh', '-c', "trap '' HUP; echo S2PID=$$; while true; do sleep 10; done"]);
  let pid = null;
  try {
    const a = await dialHello(inst.port, {});
    pid = await waitScanPid(a.frames, 'S2PID');
    if (pid !== null) sensitivePids.push(pid);
    check('S2a', "前置：A attach + 初始 OUTPUT 回读 pid（trap '' HUP 免疫夹具启动即印）",
      pid !== null, `解析=${pid !== null}`);
    if (pid === null) {
      check('S2b', 'KILL 兜底默认 5s 时序双断言', false, '前提失败：pid 未解析');
    } else {
      a.ws.close(1000);
      await waitClose(a.ws, 3000); // close 握手完成 ⇒ detach 已发生（HUP 已发、5s KILL 定时器已武装）
      await sleep(2000); // 静默窗：~2s 时点存活（5s 标称的 2.5× 裕量——更早的 KILL 即默认值漂移）
      const aliveAt2s = pgroupAlive(pid);
      const t0 = Date.now();
      const gone = await pollESRCH(pid, 12000); // 5s 到期 KILL + 调度护栏（护栏上限只防挂死）
      const elapsed = Date.now() - t0 + 2000;
      check('S2b', '时序双断言：~2s 时点存活（HUP 免疫 + KILL 未到期）+ 12s 护栏内 ESRCH 且断开至收割 ≥4s（默认 5s KILL 兜底——D-01 零配置生效）',
        aliveAt2s && gone && elapsed >= 4000,
        `静默窗存活=${aliveAt2s} ESRCH=${gone} 断开至收割≈${(elapsed / 1000).toFixed(1)}s（标称 5s）`);
    }
  } finally {
    // CI 夹具纪律：断言失败路径也不泄漏滞留进程组（ESRCH 幂等静默）
    if (pid !== null) { try { process.kill(-pid, 'SIGKILL'); } catch { /* 已消亡 */ } }
    inst.kill();
  }
  const zinst = await startWesh(['--session-mode=per-client', '--stop-timeout=0', '--', 'sh', '-c', "trap '' HUP; echo S2ZPID=$$; while true; do sleep 10; done"]);
  let zpid = null;
  try {
    // 13-01 D-02 泄漏风险 warn（validateStartup 启动期 stderr——轮询吸纳管道时序）
    let warned = false;
    const tW = Date.now();
    while (Date.now() - tW < 2000 && !warned) {
      warned = zinst.stderrText().includes('disables the SIGKILL backstop');
      if (!warned) await sleep(50);
    }
    const a = await dialHello(zinst.port, {});
    zpid = await waitScanPid(a.frames, 'S2ZPID');
    if (zpid !== null) sensitivePids.push(zpid);
    a.ws.close(1000);
    await waitClose(a.ws, 3000);
    await sleep(6000); // 6s > 5s 标称 + 1s 护栏——显式 0 被误改回默认 5s 则此处已被 KILL
    const leaked = zpid !== null && pgroupAlive(zpid);
    check('S2c', '对照：显式 --stop-timeout=0 → stderr 泄漏 warn 行 + 6s 窗后免疫进程仍存活（无 KILL 兜底——D-02 显式位尊重，泄漏形态）',
      warned && zpid !== null && leaked, `warn=${warned} 泄漏存活=${leaked}`);
  } finally {
    // 防泄漏红线（must_haves）：免疫进程 kill -9 清场——UAT 自身零残留
    if (zpid !== null) { try { process.kill(-zpid, 'SIGKILL'); } catch { /* 已消亡 */ } }
    zinst.kill();
  }
  // 场景级残留零命中断言：两免疫进程组 kill -9 + 两实例 kill 后，pgrep -f 夹具
  // 形态零匹配（wesh 进程 argv 含夹具串——kill 后异步退出窗由轮询吸纳；pgrep
  // 无匹配退出码 1，phase11 S1a 形态）。acceptance：脚本结束后无残留进程。
  let residue = true;
  const tP = Date.now();
  while (Date.now() - tP < 3000) {
    try {
      execFileSync('pgrep', ['-f', 'S2Z?PID']); // ERE：S2 + 可选 Z + PID——双夹具标签一式覆盖
      await sleep(100);
    } catch (e) {
      if (e.status === 1) { residue = false; break; } // pgrep 无匹配退出码 1
      throw e;
    }
  }
  check('S2d', 'S2 收口残留零命中：pgrep -f 免疫夹具形态零匹配（UAT 自身零泄漏——T-13-23）',
    !residue, `残留=${residue}`);
}

// ---------- S3：退出三形态 per-client 进程级 255（PC-09/accept-255 进程级映射） ----------
// 四子形态各独立实例（phase06 S3/S4/S5 的 per-client 同构——退出路径经 13-03
// pcSupervisor 第二终结源 + last-reaped-code 规则：客户端先断 → teardown SIGHUP
// 信号死亡 → exitf(-1) → Unix 截断 255）。宽限取消形态：重连 attach 取消计时
// （跨原到期点存活证明）+ 再断开新宽限到期退出（重连后退出语义不退化）。
async function s3ExitFormsPerClient() {
  console.log('S3: 退出三形态 per-client 255（--once / --exit-when-empty 裸 flag / =2s 宽限到期 + 宽限内重连取消存活）');
  {
    const inst = await startWesh(['--session-mode=per-client', '--once', '--writable', '--', 'sh']);
    try {
      const c = await dialHello(inst.port, {});
      const exitP = waitExit(inst.child, 10000);
      c.ws.close(1000);
      const proc = await exitP;
      check('S3a', '--once 唯一客户端断开 → 服务端进程退出且退出状态==255（accept-255 进程级映射）',
        proc !== null && proc.code === 255, `code=${proc?.code ?? '（未到）'}`);
    } finally {
      inst.kill();
    }
  }
  {
    const inst = await startWesh(['--session-mode=per-client', '--exit-when-empty', '--writable', '--', 'sh']);
    try {
      const c = await dialHello(inst.port, {});
      const exitP = waitExit(inst.child, 10000);
      c.ws.close(1000);
      const proc = await exitP;
      check('S3b', '--exit-when-empty 裸 flag（grace=0 立即）：断开 → 退出 255',
        proc !== null && proc.code === 255, `code=${proc?.code ?? '（未到）'}`);
    } finally {
      inst.kill();
    }
  }
  {
    const inst = await startWesh(['--session-mode=per-client', '--exit-when-empty=2s', '--writable', '--', 'sh']);
    try {
      const c = await dialHello(inst.port, {});
      const exitP = waitExit(inst.child, 10000); // 到期 ≈2s，护栏吸纳调度余量（禁精确时点断言）
      c.ws.close(1000);
      const proc = await exitP;
      check('S3c', '--exit-when-empty=2s 宽限到期（无人归）→ 退出 255（护栏 10s 内）',
        proc !== null && proc.code === 255, `code=${proc?.code ?? '（未到）'}`);
    } finally {
      inst.kill();
    }
  }
  {
    const inst = await startWesh(['--session-mode=per-client', '--exit-when-empty=2s', '--writable', '--', 'sh']);
    try {
      const c1 = await dialHello(inst.port, {});
      c1.ws.close(1000);
      await waitClose(c1.ws, 3000); // close 握手完成 ⇒ 服务端 detach 已发生（宽限计时起点已过）
      await sleep(600); // 宽限内 600ms ≪ 2s（1400ms 调度余量）
      const c2 = await dialHello(inst.port, {});
      const alive = await echoMark(c2.frames, c2.ws, 'UAT_S3_ALIVE_q4n8');
      // 取消证明：跨过原宽限到期点（c2 attach 后 3.2s > 残余宽限 ~1.4s + 1.8s 护栏）
      // 仍无 exit——若计时未被取消，进程已于 ~1.4s 前退出
      const staleExit = await waitExit(inst.child, 3200);
      check('S3d', '宽限内重连：attach 成功 + echo 探针回读 + 跨原到期点存活（宽限取消——取消形态的进程级映射）',
        c2.ws.readyState === WebSocket.OPEN && alive && staleExit === null,
        `attach=${c2.ws.readyState === WebSocket.OPEN} echo=${alive} 跨期存活=${staleExit === null}`);
      const exitP = waitExit(inst.child, 10000);
      c2.ws.close(1000);
      const proc = await exitP;
      check('S3e', '再断开后新宽限到期退出 255（重连后退出语义不退化）',
        proc !== null && proc.code === 255, `code=${proc?.code ?? '（未到）'}`);
    } finally {
      inst.kill();
    }
  }
}

// ---------- S4：Shutdown N 组（PC-09/13-04 进程级——快照逐组信号 + session_end 审计） ----------
async function s4ShutdownTwoGroups() {
  console.log('S4: Shutdown N 组（双客户端双独立 sh → SIGTERM → 双 1001 + 双 pgid ESRCH + session_end==2 全 SIGHUP + 进程退出 255）');
  const inst = await startWesh(['--session-mode=per-client', '--writable', '--', 'sh']);
  let pidA = null, pidB = null;
  try {
    const a = await dialHello(inst.port, {});
    const b = await dialHello(inst.port, {});
    pidA = await readPid(a.frames, a.ws, 'S4APID');
    pidB = await readPid(b.frames, b.ws, 'S4BPID');
    if (pidA !== null) sensitivePids.push(pidA);
    if (pidB !== null) sensitivePids.push(pidB);
    check('S4a', '前置：双客户端 attach + 各自回读 pid 且不等（双独立会话就位）',
      pidA !== null && pidB !== null && pidA !== pidB,
      `A解析=${pidA !== null} B解析=${pidB !== null} pid不等=${pidA !== pidB}`);
    if (pidA === null || pidB === null) {
      check('S4b', 'Shutdown N 组', false, '前提失败：pid 未解析');
      return;
    }
    // 收集器先于信号换装（phase07 S7 纪律——SIGTERM 后 1001 广播必达）
    const collA = collectUntilClose(a.ws, 15000);
    const collB = collectUntilClose(b.ws, 15000);
    process.kill(inst.child.pid, 'SIGTERM');
    const rA = await collA;
    const rB = await collB;
    check('S4b', 'SIGTERM 后双端各收 close 1001 且 reason 含 server_shutting_down（1001 广播全链）',
      rA.close.code === 1001 && rB.close.code === 1001
        && (rA.close.reason?.includes('server_shutting_down') ?? false)
        && (rB.close.reason?.includes('server_shutting_down') ?? false),
      `A=${rA.close.code}/${rA.close.reason?.includes('server_shutting_down') ?? false} B=${rB.close.code}/${rB.close.reason?.includes('server_shutting_down') ?? false}`);
    const proc = await waitExit(inst.child, 15000);
    check('S4c', '服务端进程退出且退出状态==255（默认 HUP stop-signal 信号死亡，last-reaped-code 规则）',
      proc !== null && proc.code === 255, `code=${proc?.code ?? '（未到）'}`);
    const goneA = await pollESRCH(pidA, 5000);
    const goneB = await pollESRCH(pidB, 5000);
    check('S4d', '两 pgid 各 5s 护栏内 ESRCH（pcSessions 快照逐组信号 + 收割完成）',
      goneA && goneB, `A=${goneA} B=${goneB}`);
    // session_end 事件数==2（13-03 happens-before 链：emit → close(waitDone) →
    // delete → Broadcast → exitf——waitExit 收码即两事件均已落流；轮询吸纳
    // stderr 管道异步送达）
    let ends = [];
    const t0 = Date.now();
    while (Date.now() - t0 < 3000) {
      ends = parseEvents(inst.stderrText()).filter((m) => m.event === 'session_end');
      if (ends.length === 2) break;
      await sleep(100);
    }
    const sigHup = ends.length === 2 && ends.every((m) => m.signal === 'SIGHUP');
    check('S4e', 'stderr session_end 事件数==2 且各 signal==SIGHUP（13-03 审计粒度 + 13-04 Shutdown 覆盖零丢失）',
      ends.length === 2 && sigHup, `事件数=${ends.length} 全SIGHUP=${sigHup}`);
  } finally {
    if (pidA !== null) { try { process.kill(-pidA, 'SIGKILL'); } catch { /* 已消亡 */ } }
    if (pidB !== null) { try { process.kill(-pidB, 'SIGKILL'); } catch { /* 已消亡 */ } }
    inst.kill();
  }
}

// ---------- S5：metrics 四计数器 + session_active 计数语义 + 零身份 label（OPS-12/D-07/D-08） ----------
async function s5MetricsPerClient() {
  console.log('S5: metrics（per-client：四计数器 series 全在 + spawn_total≥1 + session_active==1 + HELP 会话计数文案 + 零身份 label；shared 对照：四计数器恒 0 series 保留 + HELP 探活文案逐字）');
  const FOUR = ['wesh_pty_spawn_total', 'wesh_pty_spawn_failures_total', 'wesh_pty_kills_total', 'wesh_pty_spawn_throttled_total'];
  // per-client 实例：attach 1 会话 → /metrics 黑盒 scrape（无认证实例直通——D-08/D-09）
  const pc = await startWesh(['--session-mode=per-client', '--', 'sh']);
  try {
    const c = await dialHello(pc.port, {});
    const resp = await fetch(`http://127.0.0.1:${pc.port}/metrics`);
    const body = await resp.text();
    const fourAll = FOUR.every((n) => body.includes(`# HELP ${n} `));
    const spawnTotal = metricValue(body, 'wesh_pty_spawn_total');
    const sessActive = metricValue(body, 'wesh_session_active');
    check('S5a', 'per-client：四计数器 series 全在 + wesh_pty_spawn_total≥1 + wesh_session_active==1（活跃会话计数语义——D-07/D-08）',
      resp.status === 200 && fourAll && spawnTotal !== null && spawnTotal >= 1 && sessActive === 1,
      `status=${resp.status} 四series=${fourAll} spawn_total=${spawnTotal} session_active=${sessActive}`);
    const helpPc = body.includes('# HELP wesh_session_active Number of active per-client PTY sessions.');
    check('S5b', 'per-client：wesh_session_active HELP 文案为会话计数形态（文案按模式生成——D-07）',
      helpPc, `HELP命中=${helpPc}`);
    // 零身份 label：非注释样本行含 '{' 的仅 build_info（version 单 label 非身份面）
    // ——label 花括号形态零命中红线（T-13-16，08 前例形态沿用）
    const badLabels = body.split('\n').filter((l) => !l.startsWith('#') && l.includes('{') && !l.startsWith('wesh_build_info{'));
    check('S5c', '零身份 label：全部 series 样本行零 label 花括号（build_info version 豁免——红线运行时自证）',
      badLabels.length === 0, `违规行=${badLabels.length}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    pc.kill();
  }
  // shared 对照实例：四计数器恒 0 + series 保留不摘（credit_gate 恒 0 先例）+
  // HELP 探活文案逐字（shared 语义零漂移——PC-09 prohibition 对照面）
  const sh = await startWesh(['--', 'sh']);
  try {
    const c = await dialHello(sh.port, {});
    const resp = await fetch(`http://127.0.0.1:${sh.port}/metrics`);
    const body = await resp.text();
    const fourAll = FOUR.every((n) => body.includes(`# HELP ${n} `));
    const zeros = FOUR.every((n) => metricValue(body, n) === 0);
    const sessActive = metricValue(body, 'wesh_session_active');
    check('S5d', 'shared 对照：四计数器恒 0 且 series 保留不摘 + session_active==1 探活语义（shared 形态逐字未动）',
      resp.status === 200 && fourAll && zeros && sessActive === 1,
      `status=${resp.status} 四series=${fourAll} 全零=${zeros} session_active=${sessActive}`);
    const helpSh = body.includes('# HELP wesh_session_active Whether the PTY session is alive (1) or exited (0).');
    check('S5e', 'shared 对照：wesh_session_active HELP 文案为探活形态逐字（现状零漂移）',
      helpSh, `HELP命中=${helpSh}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    sh.kill();
  }
}

// ---------- S6：WESH_REMOTE_USER env 可见（SEC-09 注入链 / D-15 shared 收窄零漂移） ----------
// 回读通道：WS 内发 printenv WESH_REMOTE_USER 读 OUTPUT 结果行（命令回显不含
// 头值——值字符串只出现在结果行，锚定纪律）。C1 注入线形构造（phase07 S4c 同款）：
// undici 头值 latin1 上线，'ca\u00C2\u0085rl' 线上字节 0xC2 0x85 = UTF-8 的
// U+0085 NEL——Go 解码得 NEL，提取点 sanitizeRemoteUser 剥离 → 'carl'（13-06
// 传递链全程未触碰值本身，剥离证据即 sanitize 先于注入的结构性证明）。
async function s6WeshRemoteUserEnv() {
  console.log('S6: WESH_REMOTE_USER env 可见（per-client 携头 → env 回读含注入值 + C1 注入剥离形态；shared 对照同头无键 + 会话照常）');
  const pc = await startWesh(['--session-mode=per-client', '--writable', '--auth-header', 'X-Remote-User', '--', 'bash', '--norc', '--noprofile']);
  try {
    // ① 干净头值 'alice' → 子进程 env 回读（13-06 whitelistEnv 第三参全链）
    const c1 = await dialHello(pc.port, { headers: { 'X-Remote-User': 'alice' } });
    const base1 = c1.frames.length;
    sendInput(c1.ws, 'printenv WESH_REMOTE_USER\r');
    let val1 = false;
    const t1 = Date.now();
    while (Date.now() - t1 < 5000 && !val1) {
      val1 = outputText(c1.frames, base1).includes('alice');
      if (!val1) await sleep(50);
    }
    check('S6a', 'per-client 携 X-Remote-User 头 → 子进程 env 回读含 WESH_REMOTE_USER=alice（printenv 结果行——注入链全通）',
      val1, `回读=${val1}`);
    c1.ws.close(1000);
    await waitClose(c1.ws, 3000);
    // ② C1 控制字符注入头（NEL 线形）→ 回读值为剥离产物 carl 且控制字符零出现
    const NEL_WIRE = 'ca\u00C2\u0085rl';
    const c2 = await dialHello(pc.port, { headers: { 'X-Remote-User': NEL_WIRE } });
    const base2 = c2.frames.length;
    sendInput(c2.ws, 'printenv WESH_REMOTE_USER\r');
    let val2 = false;
    const t2 = Date.now();
    while (Date.now() - t2 < 5000 && !val2) {
      const out = outputText(c2.frames, base2);
      val2 = out.includes('carl') && !out.includes('\u0085');
      if (!val2) await sleep(50);
    }
    check('S6b', 'C1 注入头值（NEL 线形）→ env 回读值为剥离产物 carl 且控制字符零出现（sanitize 先于注入——D-19 防线延伸到 env 面）',
      val2, `剥离回读=${val2}`);
    c2.ws.close(1000);
    await waitClose(c2.ws, 3000);
  } finally {
    pc.kill();
  }
  // ③ shared 对照：同携头 attach → env 无该键（D-15 收窄语义不变——whitelistEnv
  // 空串不出键的结构性保证）；printenv 零输出以 echo 标记收口（标记到达 ⇒
  // printenv 结果行若有必已到达——程序序锚定，替代固定 sleep 缺席断言）
  const sh = await startWesh(['--writable', '--auth-header', 'X-Remote-User', '--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(sh.port, { headers: { 'X-Remote-User': 'alice' } });
    const base = c.frames.length;
    sendInput(c.ws, 'printenv WESH_REMOTE_USER\r');
    sendInput(c.ws, 'echo UAT_S6_DONE_j5m2\r');
    let done = false;
    const t0 = Date.now();
    while (Date.now() - t0 < 5000 && !done) {
      done = /\nUAT_S6_DONE_j5m2/.test(outputText(c.frames, base)); // 结果行锚定（排除命令回显行）
      if (!done) await sleep(50);
    }
    const noKey = done && !outputText(c.frames, base).includes('alice');
    check('S6c', 'shared 对照：同携头 attach → env 无 WESH_REMOTE_USER 键（printenv 零输出）+ echo 标记收口到达（D-15 收窄零漂移）',
      noKey, `标记到达=${done} 无键=${noKey}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    sh.kill();
  }
}

// 输出自净断言（phase12.mjs:733-739 逐字——红线由注释纪律升级为运行时自证）：
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
// 退出码（phase12.mjs 收口模式逐字；与头注释六场景清单逐一对账）
const scenarios = [s1ChurnThrottleXff, s2KillBackstopDefault5s, s3ExitFormsPerClient, s4ShutdownTwoGroups, s5MetricsPerClient, s6WeshRemoteUserEnv];
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
