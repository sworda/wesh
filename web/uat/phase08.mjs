// Phase 8 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch）。
// 覆盖 08-01..08-04 四 plan 的可观测性交付对真实二进制的全链断言（OPS-06/07/08、
// D-07/D-08/D-09/D-10/D-11/D-18/D-19/D-20/D-21/D-22/D-23）：S1 健康检查
// （无认证 200+四字段 / 凭据模式无 Authorization 头 200 + 对照 GET / 401 例外不蔓延 /
// bp=/wesh 下 /healthz 200 且 /wesh/healthz 404 根路径固定 / POST 405+Allow:GET）、
// S2 metrics 认证闸两态（凭据模式无凭据 401 / 正确凭据 200 + Content-Type 含
// text/plain 与 version=0.0.4；--no-auth 直通 200）、S3 metrics exposition
// （17 series 名全在场 + connected/total==2 + pty_output>0 + ws_sent≥pty_output +
// session_active==1 + build_info version="dev"；bp=/wesh 下 /metrics 200 且
// /wesh/metrics 404）、S4 关停中 503 draining（trap 忽略 HUP + --stop-timeout 3s
// 拉宽窗口 → SIGTERM 立即轮询 /healthz 断言 503 status=draining → 进程 255 退出）、
// S5 审计事件 JSON 行检索（auth_failed 无 user/username 键 / throttled 携
// retry_after≥1 / attach+detach 各恰 1 条 client_id 相等 reason=normal /
// session_end exit_code=42 duration_seconds>0 无 signal 键）、S6 控制字符剥离回归
// （NEL 线形探针 remote_user=alice / XFF 链首注入 remote 逐码点无 C0/C1/DEL /
// 对照组无 remote_user 键）。
//
// 红线（phase07.mjs:14-17 纪律逐字沿用）：share token/凭据值只作断言材料，永不进入
// check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/退出码/event 名。
// S5 的错凭据探针串与 share token 同口径入 sensitiveTokens 闭包数组，
// assertOutputClean 运行时自证零命中。
//
// 单次语义纪律（phase07.mjs:19-21 逐字沿用）：S4 关停与 S5 子进程退出场景的服务端
// 进程退出是特性不是回归——child 'exit' 事件即断言通道（waitExit helper），spawn
// 实例 SIGKILL 收口仅用于未预期退出场景的清理。
//
// 运行：node web/uat/phase08.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
// 调试：PHASE08_ONLY=S1,S3 node web/uat/phase08.mjs（场景过滤，仅调试用——提交形态
// 恒为全场景开启）。
import { spawn } from 'node:child_process';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
const SUBPROTOCOL = 'wesh.v1';

// UAT 专用凭据（phase03.mjs 同款；值不入任何输出——红线）；basicAuthHeader 仅作请求构造材料。
const UAT_CREDENTIAL = 'uat:uat-pass-x9';
const basicAuthHeader = () => 'Basic ' + Buffer.from(UAT_CREDENTIAL).toString('base64');
// 按值构造 Basic 头（值同样只作请求构造材料——红线同口径）
const basicHeader = (cred) => 'Basic ' + Buffer.from(cred).toString('base64');

const enc = new TextEncoder();
const dec = new TextDecoder();
const concat = (...parts) => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) { out.set(p, off); off += p.length; }
  return out;
};
// Hello 载荷 {version,cols,rows[,ticket]}；ticket undefined 时 JSON 省略字段
// （omitempty 对称形态：无认证模式前端不出 ticket 键）。
const helloFrame = ({ ticket, version = SUBPROTOCOL, cols = 80, rows = 24 } = {}) =>
  concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify(
    ticket === undefined
      ? { version, cols, rows }
      : { version, cols, rows, ticket })));

const results = [];
// 全部已发 detail 收集（assertOutputClean 遍历材料——WR-02 运行时自净断言）
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

// parseEvents：stderr 混合流按行解析 JSON 事件（08-01 D-13 迁移后事件为 slog JSON
// 单行）——滤非 '{' 起始行（启动行/警告行等人文本成员不算事件）；'{' 起始行非法
// JSON 即抛错（带行号与行首 120 字符截断）。事件值只作断言材料——detail 只打
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

// startWesh 解析 stdout 时把分享链接 token 留入本闭包数组（只作 assertOutputClean
// 断言材料）——红线：token 值永不进 check detail/控制台输出/汇总行。S5 的错凭据
// 探针串同口径入本数组（值同样永不进任何输出）。
const sensitiveTokens = [];

// 分享链接 URL → token（/s/{token}/ 路径段；值只作断言材料——红线）
const tokenFromUrl = (url) => /\/s\/([^/]+)\//.exec(url)[1];

// WR-02（06-REVIEW）：启动超时 reject 消息脱敏——--credential 后随值（空格与 = 两形态）
// 替换为 <redacted>。场景异常通道（尾部 catch 原样打印 e.message）经 emittedDetails
// 进 assertOutputClean 扫描面，argv 原样回显会把凭据值明文送进控制台/CI 日志。
const redactArgs = (args) => args.map((a, i) => {
  if (a.startsWith('--credential=')) return '--credential=<redacted>';
  if (i > 0 && args[i - 1] === '--credential') return '<redacted>';
  return a;
}).join(' ');

// 启动 wesh 实例，返回 { port, scheme, shareRO, shareRW, stderrText, stdoutText, kill, child }。
// （phase07.mjs 逐字沿用——本 phase 无 unix socket 场景，unix 解析分支保持不消费。）
// opts.defaultListen（默认 true）：前置 --bind 127.0.0.1 --port 0（loopback 随机端口，
// 与用户服务零干扰）；false 时 argv 原样。
// opts.env（默认 process.env）：子进程环境。
// stderr 持续捕获（JSON 事件行/警告行/panic 断言通道——S4/S5/S6 核心消费点）。
function startWesh(args, { defaultListen = true, unix = false, env } = {}) {
  return new Promise((resolve, reject) => {
    const argv = defaultListen ? ['--bind', '127.0.0.1', '--port', '0', ...args] : args;
    const child = spawn(WESH, argv, { stdio: ['ignore', 'pipe', 'pipe'], env: env ?? process.env });
    let stderr = '';
    let stdoutBuf = '';
    let settling = false;
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${redactArgs(argv)}; stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      stdoutBuf += d.toString();
      if (settling) return;
      if (unix) {
        // D-12 unix 形态：地址行 unix:// 前缀 + 退化单行，无 share token 可收集
        if (stdoutBuf.includes('listening on unix://')) {
          settling = true;
          clearTimeout(to);
          setTimeout(() => {
            const unixPath = /listening on (unix:\/\/\S+)/.exec(stdoutBuf)?.[1] ?? null;
            resolve({ unixPath, stderrText: () => stderr, stdoutText: () => stdoutBuf, kill: () => child.kill('SIGKILL'), child });
          }, 50);
        }
        return;
      }
      // scheme 感知启动行解析（照 phase07.mjs 形态）；ro 行齐备后 50ms 落定窗吸纳 rw 行
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
          resolve({ port: Number(m[2]), scheme: m[1], shareRO, shareRW, stderrText: () => stderr, stdoutText: () => stdoutBuf, kill: () => child.kill('SIGKILL'), child });
        }, 50);
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// 建立 WS 连接并完成 Hello 握手（可携 ticket、可定尺寸）；返回 { ws, frames }，frames 持续累积。
// opts.path（默认 '/ws'）；opts.headers（默认无）：S6 auth-header/XFF 场景的反代头注入——
// Node >= 22 原生 WebSocket 第二参 { headers, protocols } 形态（07-07 本机探针实证：
// 自定义头与 C1 控制字符均可传输）。无 headers 时保持数组形态第二参（phase07.mjs 逐字）。
function dialHello(port, { ticket, cols = 80, rows = 24, path = '/ws', headers } = {}) {
  return new Promise((resolve, reject) => {
    const url = `ws://127.0.0.1:${port}${path}`;
    const ws = headers === undefined
      ? new WebSocket(url, [SUBPROTOCOL])
      : new WebSocket(url, { headers, protocols: [SUBPROTOCOL] });
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame({ ticket, cols, rows }));
    ws.onerror = () => reject(new Error('WS 连接失败'));
    // IN-04 总超时 watchdog：被测二进制挂死（Welcome 永不到达）时 10s 拒绝而非永久悬挂
    const watchdog = setTimeout(() => {
      clearInterval(poll);
      reject(new Error('握手总超时：10s 未收到 Welcome'));
    }, 10000);
    // Welcome 到达即视为握手完成
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

// waitExit：child 'exit' 事件决议 {code, signal}——S4/S5 的 wesh 进程退出断言通道
//（本 phase 场景的服务端退出是特性不是回归）。恒带超时护栏：被测二进制挂死时护栏
// 到期 resolve(null) 由断言转 FAIL，而非无限等待（phase07.mjs 同款纪律）。
function waitExit(child, timeoutMs) {
  return new Promise((resolve) => {
    const to = setTimeout(() => resolve(null), timeoutMs);
    child.once('exit', (code, signal) => { clearTimeout(to); resolve({ code, signal }); });
  });
}

// 帧收集器 collectUntilClose(ws)（phase07.mjs 逐字）：换装 onmessage/onclose 为本收集器，
// close 到达时决议 {frames, close:{code,reason}}。
function collectUntilClose(ws, timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    const frames = [];
    const to = setTimeout(() => reject(new Error(`collectUntilClose 超时：${timeoutMs}ms 未收到 close`)), timeoutMs);
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onclose = (ev) => { clearTimeout(to); resolve({ frames, close: { code: ev.code, reason: ev.reason } }); };
  });
}

const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');

// INPUT 帧发送 helper（phase07.mjs 逐字——S3 echo 链路消费）
const sendInput = (ws, text) => ws.send(concat(new Uint8Array([INPUT]), enc.encode(text)));

// echo 链路轮询：frames[fromIdx..] 的 OUTPUT 文本含 marker 即 true（5s 默认护栏，
// phase07.mjs 同款 50ms 爬梯）
async function waitOutput(frames, fromIdx, marker, timeoutMs = 5000) {
  const t0 = Date.now();
  while (Date.now() - t0 < timeoutMs) {
    if (outputText(frames, fromIdx).includes(marker)) return true;
    await sleep(50);
  }
  return false;
}

// 事件流轮询：parseEvents(stderrText()) 出现满足 pred 的事件即返回该事件（2s 默认
// 护栏）——stderr 捕获是异步流，attach/detach 等事件的落流晚于 WS 握手/关闭决议点，
// 直接读取有竞态。事件值只作断言材料（红线保持）。
async function waitEvent(inst, pred, timeoutMs = 2000) {
  const t0 = Date.now();
  while (Date.now() - t0 < timeoutMs) {
    const hit = parseEvents(inst.stderrText()).find(pred);
    if (hit !== undefined) return hit;
    await sleep(50);
  }
  return null;
}

// /healthz body 恰四键形状断言（D-10 键集白名单 + 类型/值锁——08-03 getHealthz
// 同构形态）：status=="ok"、clients 为数字、max_clients==32、session_active==true。
const healthzOkShape = (b) =>
  b !== null && typeof b === 'object'
  && Object.keys(b).sort().join(',') === 'clients,max_clients,session_active,status'
  && b.status === 'ok' && typeof b.clients === 'number'
  && b.max_clients === 32 && b.session_active === true;

// ---------- S1：健康检查（OPS-06：D-07 免认证窄例外 / D-09 根路径固定 / D-10 四字段 JSON / 405 成对注册） ----------
async function s1Healthz() {
  console.log('S1: 健康检查（无认证 200+四字段 / 凭据模式无头 200 + 对照 GET / 401 / bp=/wesh 根路径固定 / POST 405+Allow:GET）');
  // (a) 无认证实例：GET /healthz → 200 + Content-Type application/json + body 恰四键
  const instA = await startWesh(['--', 'bash', '--norc', '--noprofile']);
  try {
    const resp = await fetch(`http://127.0.0.1:${instA.port}/healthz`);
    const body = await resp.json().catch(() => null);
    check('S1a', '无认证实例 GET /healthz → 200 + application/json + body 恰四键（status=ok/clients 数字/max_clients=32/session_active=true）',
      resp.status === 200 && (resp.headers.get('Content-Type') ?? '').includes('application/json') && healthzOkShape(body),
      `status=${resp.status} json=${(resp.headers.get('Content-Type') ?? '').includes('application/json')} 四键形状=${healthzOkShape(body)}`);
    // (d) POST /healthz → 405 + Allow: GET（方法模式 + path-only fallback 成对注册，
    // RESEARCH Pitfall 7 防线——内建 405 会被 "/" 子树吞掉）
    const respPost = await fetch(`http://127.0.0.1:${instA.port}/healthz`, { method: 'POST' });
    check('S1d', 'POST /healthz → 405 + Allow: GET',
      respPost.status === 405 && (respPost.headers.get('Allow') ?? '').includes('GET'),
      `status=${respPost.status} Allow含GET=${(respPost.headers.get('Allow') ?? '').includes('GET')}`);
    await respPost.text();
  } finally {
    instA.kill();
  }

  // (b) 凭据实例：无 Authorization 头 GET /healthz → 200 同形态（D-07 整站 Basic 闸
  // 唯一窄例外）；对照 GET / 无凭据 → 401（例外不蔓延）。排序即解零 pacing：
  // /healthz 免认证不过节流闸先行，401 负面对照排最后（fail#1 +1s 窗口无后续消费者）。
  const instB = await startWesh(['--credential', UAT_CREDENTIAL, '--', 'bash', '--norc', '--noprofile']);
  try {
    const respHz = await fetch(`http://127.0.0.1:${instB.port}/healthz`);
    const bodyHz = await respHz.json().catch(() => null);
    const respRoot = await fetch(`http://127.0.0.1:${instB.port}/`);
    check('S1b', '凭据模式无 Authorization 头 GET /healthz → 200 同形态（D-07 例外），对照 GET / → 401（例外不蔓延）',
      respHz.status === 200 && healthzOkShape(bodyHz) && respRoot.status === 401,
      `healthz=${respHz.status} 四键形状=${healthzOkShape(bodyHz)} root=${respRoot.status}`);
    await respRoot.text();
  } finally {
    instB.kill();
  }

  // (c) bp=/wesh 实例（无认证模式精确码，D-09）：GET /healthz → 200（根路径固定不受
  // bp 影响），GET /wesh/healthz → 404（bp 子树内无此路径——拒绝双挂）。
  const instC = await startWesh(['--base-path', '/wesh', '--', 'bash', '--norc', '--noprofile']);
  try {
    const respRoot = await fetch(`http://127.0.0.1:${instC.port}/healthz`);
    const okRoot = respRoot.status === 200 && healthzOkShape(await respRoot.json().catch(() => null));
    const respBp = await fetch(`http://127.0.0.1:${instC.port}/wesh/healthz`);
    check('S1c', 'bp=/wesh：GET /healthz → 200（根路径固定）且 GET /wesh/healthz → 404（拒绝双挂）',
      okRoot && respBp.status === 404,
      `根=${okRoot} bp内=${respBp.status}`);
    await respBp.text();
  } finally {
    instC.kill();
  }
}

// ---------- S2：metrics 认证闸两态（OPS-07 D-08：凭据模式过 Basic 闸 / --no-auth 直通） ----------
async function s2MetricsAuthGate() {
  console.log('S2: metrics 认证闸（凭据模式：正确凭据 200 + Content-Type 含 text/plain/version=0.0.4，无凭据 401；--no-auth 直通 200）');
  // 凭据实例：排序即解零 pacing（phase07 S1 先例）——正确凭据先行（recordSuccess
  // 清零节流），无凭据 401 负面对照排最后（fail#1 +1s 窗口无后续消费者）。
  const instA = await startWesh(['--credential', UAT_CREDENTIAL, '--', 'bash', '--norc', '--noprofile']);
  try {
    const respOk = await fetch(`http://127.0.0.1:${instA.port}/metrics`, { headers: { Authorization: basicAuthHeader() } });
    const ct = respOk.headers.get('Content-Type') ?? '';
    await respOk.text();
    check('S2a', '凭据模式正确凭据 GET /metrics → 200 且 Content-Type 含 text/plain 与 version=0.0.4（D-01 规范条款）',
      respOk.status === 200 && ct.includes('text/plain') && ct.includes('version=0.0.4'),
      `status=${respOk.status} text/plain=${ct.includes('text/plain')} v004=${ct.includes('version=0.0.4')}`);
    const respNo = await fetch(`http://127.0.0.1:${instA.port}/metrics`);
    check('S2b', '凭据模式无凭据 GET /metrics → 401（认证闸跟随，D-08——metrics 泄漏服务行为轮廓）',
      respNo.status === 401, `status=${respNo.status}`);
    await respNo.text();
  } finally {
    instA.kill();
  }
  // --no-auth 实例：无认证闸，/metrics 直通 200（暴露面明示义务随 README 运维节）
  const instB = await startWesh(['--no-auth', '--', 'bash', '--norc', '--noprofile']);
  try {
    const resp = await fetch(`http://127.0.0.1:${instB.port}/metrics`);
    const body = await resp.text();
    check('S2c', '--no-auth 实例 GET /metrics 直通 200（无凭据要求，D-08 自然免）',
      resp.status === 200 && body.includes('# HELP wesh_clients_connected '),
      `status=${resp.status} exposition=${body.includes('# HELP wesh_clients_connected ')}`);
  } finally {
    instB.kill();
  }
}

// ---------- S3：metrics exposition 指标 + bp 固定（OPS-07：17 series 契约 + 数值锁 + D-09 根路径固定） ----------
// 17 series 契约清单（metrics.go 输出序逐字——采集契约一次性锁死，08-04 metricsSeries17 镜像）
const SERIES17 = [
  'wesh_clients_connected', 'wesh_clients_total', 'wesh_clients_kicked_total',
  'wesh_session_active', 'wesh_outbox_depth_bytes_max', 'wesh_outbox_depth_bytes_sum',
  'wesh_pty_output_bytes_total', 'wesh_ws_sent_bytes_total', 'wesh_ws_recv_bytes_total',
  'wesh_auth_failed_total', 'wesh_auth_throttled_total',
  'wesh_input_rate_dropped_total', 'wesh_input_queue_dropped_total',
  'wesh_credit_gate_transitions_total', 'wesh_goroutines', 'wesh_mem_alloc_bytes',
  'wesh_build_info',
];
// metricValue：样本行 = name + 空格 + 整数值（name+空格前缀精确名匹配不撞前缀族，
// 08-04 metricSample 同构）；label 形态行（build_info）不适用——逐字断言代替。
const metricValue = (body, name) => {
  for (const line of body.split('\n')) {
    if (line.startsWith(name + ' ')) {
      const v = Number(line.slice(name.length).trim());
      return Number.isFinite(v) ? v : null;
    }
  }
  return null;
};
async function s3MetricsExposition() {
  console.log('S3: metrics exposition（双客户端 /bin/cat 回显驱动：17 series 全在场 + connected/total==2 + pty_output>0 + ws_sent≥pty_output + session_active==1 + build_info version="dev"；bp 固定）');
  const instA = await startWesh(['--writable', '--', '/bin/cat']);
  try {
    // 双客户端 dialHello（Welcome 到达即已注册——05-10 升档序列）；默认 owner 策略
    // 首端 A 可写，INPUT 驱 /bin/cat 回显（零 pre-attach 输出，08-04 裁决——回显
    // 观测即 pty_output 已计数的同步边）
    const a = await dialHello(instA.port, {});
    const b = await dialHello(instA.port, {});
    const base = a.frames.length;
    sendInput(a.ws, 'UAT_S3_MARK\r');
    const echoed = await waitOutput(a.frames, base, 'UAT_S3_MARK');
    const resp = await fetch(`http://127.0.0.1:${instA.port}/metrics`);
    const body = await resp.text();
    const allPresent = SERIES17.every((n) => body.includes(`# HELP ${n} `));
    const connected = metricValue(body, 'wesh_clients_connected');
    const total = metricValue(body, 'wesh_clients_total');
    const ptyOut = metricValue(body, 'wesh_pty_output_bytes_total');
    const wsSent = metricValue(body, 'wesh_ws_sent_bytes_total');
    const sessActive = metricValue(body, 'wesh_session_active');
    const sentGePty = wsSent !== null && ptyOut !== null && wsSent >= ptyOut;
    check('S3a', '17 series 名全在场（HELP 行逐字）且数值锁：connected==2/total==2/pty_output>0/ws_sent≥pty_output/session_active==1',
      resp.status === 200 && echoed && allPresent && connected === 2 && total === 2
      && ptyOut !== null && ptyOut > 0 && sentGePty && sessActive === 1,
      `status=${resp.status} echo=${echoed} series全=${allPresent} connected=${connected} total=${total} pty>0=${ptyOut !== null && ptyOut > 0} sent≥pty=${sentGePty} sess=${sessActive}`);
    check('S3b', 'wesh_build_info 行含 version="dev" 且 gauge=1（开发构建 version var 透传，Options 单一通道）',
      body.includes('wesh_build_info{version="dev"} 1'),
      `build_info行=${body.includes('wesh_build_info{version="dev"} 1')}`);
    a.ws.close();
    b.ws.close();
    await Promise.all([waitClose(a.ws, 3000), waitClose(b.ws, 3000)]);
  } finally {
    instA.kill();
  }
  // bp=/wesh 实例：/metrics 根路径固定 200，/wesh/metrics 404（无认证模式精确码，
  // D-09——采集器直连后端端口，路径恒定可写死进 scrape 静态配置；拒绝双挂）
  const instB = await startWesh(['--base-path', '/wesh', '--', 'bash', '--norc', '--noprofile']);
  try {
    const respRoot = await fetch(`http://127.0.0.1:${instB.port}/metrics`);
    await respRoot.text();
    const respBp = await fetch(`http://127.0.0.1:${instB.port}/wesh/metrics`);
    check('S3c', 'bp=/wesh：GET /metrics → 200（根路径固定）且 GET /wesh/metrics → 404（拒绝双挂）',
      respRoot.status === 200 && respBp.status === 404,
      `根=${respRoot.status} bp内=${respBp.status}`);
    await respBp.text();
  } finally {
    instB.kill();
  }
}

// ---------- S4：关停中 503 draining（OPS-06 D-11：SIGTERM → draining 置位 → 窗口内 503 → 进程退出） ----------
async function s4Draining503() {
  console.log('S4: 关停中 503 draining（trap 忽略 HUP + --stop-timeout 3s 拉宽窗口 → SIGTERM 立即轮询 /healthz 503 status=draining → 退出 255）');
  // 窗口时序确定性化（08-RESEARCH OQ3 定案 + 08-03 冒烟同款夹具 + phase07 S5a 先例）：
  // trap "" HUP 整组免疫（SIG_IGN 跨 exec 持久，07-04 实证）使子进程挺过 stop-signal；
  // Shutdown 的 stop-signal 序列为同步 sleep 形态（07-05）——stop-timeout 3s 内进程
  // 存活且 draining 已置位（Shutdown 首行），draining 窗口 ≈3s，轮询确定性成立；
  // 3s 后补 SIGKILL → lifecycle 收口退出 255（accept-255 同源）。
  const inst = await startWesh(['--stop-timeout', '3s', '--', 'sh', '-c', 'trap "" HUP; sleep 100']);
  try {
    const c = await dialHello(inst.port, {});
    // SIGTERM 发送点在 dialHello 完成后——握手完成 ⇒ attach 已登记（1001 广播有受众）
    process.kill(inst.child.pid, 'SIGTERM');
    // 立即轮询 /healthz（窗口 ≈3s，50ms 爬梯，2.5s 护栏——窗口内必观测）
    let drain = null;
    const t0 = Date.now();
    while (Date.now() - t0 < 2500 && drain === null) {
      try {
        const resp = await fetch(`http://127.0.0.1:${inst.port}/healthz`);
        const body = await resp.json().catch(() => null);
        if (resp.status === 503 && body !== null && body.status === 'draining') {
          drain = { keys: Object.keys(body).sort().join(',') };
        }
      } catch { /* 连接拒绝 = 窗口已过——继续轮询至护栏由断言转 FAIL */ }
      if (drain === null) await sleep(50);
    }
    check('S4a', 'SIGTERM 后轮询窗口内 /healthz 出现 503 且 body status=="draining" 恰四键（D-11 探活翻转，反代不再向将死实例导新流）',
      drain !== null && drain.keys === 'clients,max_clients,session_active,status',
      `503观测=${drain !== null} 四键=${drain !== null && drain.keys === 'clients,max_clients,session_active,status'}`);
    const proc = await waitExit(inst.child, 15000);
    check('S4b', 'wesh 15s 护栏内退出且退出状态 255（3s 宽限补 SIGKILL → lifecycle 收口 accept-255 同源）',
      proc !== null && proc.code === 255, `code=${proc?.code ?? '（未到）'}`);
  } finally {
    inst.kill();
  }
}

// ---------- S5：审计事件 JSON 行检索（OPS-08：D-18 schema / D-20 client_id / D-21 reason / D-22 session_end / D-23 认证字段边界） ----------
async function s5AuditEvents() {
  console.log('S5: 审计事件（错凭据 401→auth_failed 无用户名 / 连发 429→throttled retry_after≥1 / attach+detach client_id 关联 reason=normal / exit 42→session_end 三字段）');
  // 错凭据探针值同口径入 sensitiveTokens 闭包数组（值永不进任何输出——红线）
  const WRONG = 'uat:wrong-pass-probe-z3';
  sensitiveTokens.push(WRONG);
  const instA = await startWesh(['--credential', UAT_CREDENTIAL, '--', 'bash', '--norc', '--noprofile']);
  try {
    // 错凭据打 401（fail#1 → notBefore=+1s）；快速连发撞窗 → 429 + Retry-After
    //（phase03 场景 2 确定性形态：429 不 recordFail 不延长窗口，无需 sleep）
    const resp401 = await fetch(`http://127.0.0.1:${instA.port}/api/attach`, {
      method: 'POST', headers: { Authorization: basicHeader(WRONG) },
    });
    await resp401.text();
    const statuses = [];
    let retryHdr = false;
    for (let i = 0; i < 4; i++) {
      const resp = await fetch(`http://127.0.0.1:${instA.port}/api/attach`, {
        method: 'POST', headers: { Authorization: basicHeader(WRONG) },
      });
      statuses.push(resp.status);
      if (resp.status === 429 && resp.headers.get('Retry-After') !== null) retryHdr = true;
      await resp.text();
    }
    check('S5a', '错凭据 401 → 快速连发 ×4 撞节流窗 429 + Retry-After（爬梯确定性形态）',
      resp401.status === 401 && statuses.every((s) => s === 429) && retryHdr,
      `首=${resp401.status} 连发=${statuses.join(',')} retryHdr=${retryHdr}`);
    await sleep(1150); // fail#1 窗口 1s 真实过窗，否则下一请求确定性 429（phase03 纪律）
    // 正确凭据 → ticket → dialHello → close（attach/detach 事件对驱动）
    const respOk = await fetch(`http://127.0.0.1:${instA.port}/api/attach`, {
      method: 'POST', headers: { Authorization: basicAuthHeader() },
    });
    const bodyOk = respOk.status === 200 ? await respOk.json() : {};
    const c = await dialHello(instA.port, { ticket: bodyOk.ticket });
    c.ws.close();
    await waitClose(c.ws, 3000);
    // 事件断言（waitEvent 等 detach 落流——stderr 异步捕获竞态防线）；detail 只打
    // event 名/布尔/数值形状（token/凭据值红线保持）
    const detachSeen = await waitEvent(instA, (m) => m.event === 'detach');
    const evs = parseEvents(instA.stderrText());
    const authFailed = evs.filter((m) => m.event === 'auth_failed');
    const throttled = evs.filter((m) => m.event === 'throttled');
    const attaches = evs.filter((m) => m.event === 'attach');
    const detaches = evs.filter((m) => m.event === 'detach');
    const noUsername = authFailed.length > 0 && authFailed.every((m) => !('user' in m) && !('username' in m));
    check('S5b', 'auth_failed 事件在场且零 user/username 键（D-23/SEC-01 红线：凭据任何形态含用户名永不入日志）',
      noUsername, `auth_failed=${authFailed.length} 无用户名键=${noUsername}`);
    const thOk = throttled.length > 0 && throttled.every((m) => typeof m.retry_after === 'number' && m.retry_after >= 1);
    check('S5c', 'throttled 事件在场且携 retry_after≥1（D-23 爆破节奏排查字段，与 Retry-After 头同值）',
      thOk, `throttled=${throttled.length} retry_after≥1=${thOk}`);
    const idLinked = attaches.length === 1 && detaches.length === 1
      && attaches[0].client_id === detaches[0].client_id;
    check('S5d', 'attach/detach 各恰 1 条且 client_id 相等、detach reason=="normal"（D-20 关联检索 / D-21 单事件四值）',
      detachSeen !== null && idLinked && detaches[0].reason === 'normal',
      `attach=${attaches.length} detach=${detaches.length} client_id关联=${idLinked} reason=normal=${detaches[0]?.reason === 'normal'}`);
  } finally {
    instA.kill();
  }

  // 独立实例：子进程 exit 42 → session_end 三字段（D-22 单事件回答「活多久、怎么死的」）
  const instB = await startWesh(['--', 'bash', '-c', 'exit 42']);
  try {
    const proc = await waitExit(instB.child, 10000);
    // 速退竞态兜底：exit 事件可能先于监听挂载（startWesh 落定窗内进程已退出）——
    // child.exitCode 为 Node 侧已退出事实的冗余通道
    const code42 = proc !== null ? proc.code : instB.child.exitCode;
    const end = await waitEvent(instB, (m) => m.event === 'session_end');
    const okEnd = end !== null && end.exit_code === 42
      && typeof end.duration_seconds === 'number' && end.duration_seconds > 0
      && !('signal' in end);
    check('S5e', '子进程 exit 42 → session_end 携 exit_code==42、duration_seconds>0、无 signal 键（D-22 仅信号死亡出键）；wesh 按子进程退出码 42 退出',
      code42 === 42 && okEnd,
      `进程code=${code42 ?? '（未到）'} exit_code42=${end?.exit_code === 42} dur>0=${end !== null && end.duration_seconds > 0} 无signal键=${end !== null && !('signal' in end)}`);
  } finally {
    instB.kill();
  }
}

// ---------- S6：控制字符剥离回归（OPS-08 D-19 双闸：remote_user/remote 的 C0/C1/DEL 剥离——JSON 化不消除 C1 穿透，RESEARCH Pitfall 5） ----------
async function s6ControlCharStrip() {
  console.log('S6: 控制字符剥离（NEL 线形探针 remote_user=alice / XFF 链首注入 remote 逐码点无 C0/C1/DEL / 对照组无 remote_user 键）');
  // 线形构造（07-07 本机探针实证逐字沿用）：undici 头值按 latin1 编码上线——发
  // JS 'ali\u00C2\u0085ce'（U+00C2 U+0085 两码点）上线字节 = 0xC2 0x85 = UTF-8 客户端
  //（Go http/curl）发送 'ali\u0085ce' 的等价线形——Go 解码得 U+0085 NEL，按 D-19 剥离。
  const NEL_WIRE = 'ali\u00C2\u0085ce';
  // 逐码点属性断言：无 C0（≤0x1f）/DEL（0x7f）/C1（0x80-0x9f）rune（proxy_test.go
  // TestRemoteSanitize 属性断言的 UAT 镜像）
  const noCtl = (s) => typeof s === 'string' && [...s].every((ch) => {
    const cp = ch.codePointAt(0);
    return !(cp <= 0x1f || cp === 0x7f || (cp >= 0x80 && cp <= 0x9f));
  });
  const instA = await startWesh(['--auth-header', 'X-Remote-User', '--', 'bash', '--norc', '--noprofile']);
  try {
    // ① NEL 探针入 X-Remote-User：attach 事件 remote_user 剥离后 == "alice" 保留
    const c1 = await dialHello(instA.port, { headers: { 'X-Remote-User': NEL_WIRE } });
    const evUser = await waitEvent(instA, (m) => m.event === 'attach' && 'remote_user' in m);
    c1.ws.close();
    await waitClose(c1.ws, 3000);
    const nelLeaked = instA.stderrText().includes('\u0085');
    check('S6a', 'NEL 线形探针：事件行 JSON 解析后 remote_user=="alice"（C1 剥离证据）且 stderr 零 NEL 泄漏',
      evUser !== null && evUser.remote_user === 'alice' && !nelLeaked,
      `remote_user=alice=${evUser?.remote_user === 'alice'} NEL缺席=${!nelLeaked}`);
    // ② XFF 链首注入同类探针：remote 字段值逐码点无 C0/C1/DEL（D-19 双闸语义——
    // ParseIP 第一道结构性拒（中段 NEL 非法 → 回退 loopback 对端键，08-02 实证
    // 受力面）+ sanitize 纵深第二道；断言面 = 输出无控制字符）
    const c2 = await dialHello(instA.port, { headers: { 'X-Forwarded-For': NEL_WIRE } });
    const evXff = await waitEvent(instA, (m) => m.event === 'attach' && !('remote_user' in m));
    c2.ws.close();
    await waitClose(c2.ws, 3000);
    check('S6b', 'XFF 链首注入同类探针：attach 事件 remote 字段值逐码点无 C0/C1/DEL rune（D-19 双闸语义）',
      evXff !== null && noCtl(evXff.remote),
      `remote洁净=${evXff !== null && noCtl(evXff.remote)}`);
  } finally {
    instA.kill();
  }

  // ③ 对照组：未配置 --auth-header → 同请求事件无 remote_user 键（D-20 单一信任闸
  // 零双轨；事件集非空前置防空捕获假绿——08-01 登记纪律）
  const instB = await startWesh(['--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(instB.port, { headers: { 'X-Remote-User': 'alice' } });
    c.ws.close();
    await waitClose(c.ws, 3000);
    await waitEvent(instB, (m) => m.event === 'detach');
    const evs = parseEvents(instB.stderrText());
    const noUserKey = evs.length > 0 && evs.every((m) => !('remote_user' in m));
    check('S6c', '对照组未配置 auth-header：全部事件无 remote_user 键（头值完全忽略）',
      noUserKey, `事件数=${evs.length} 无remote_user键=${noUserKey}`);
  } finally {
    instB.kill();
  }
}

// 输出自净断言（WR-02/review #7 形态——红线由注释纪律升级为运行时自证）：遍历全部已发
// detail，断言不含 UAT_CREDENTIAL 值、任一 share token 值（含 '/s/' 链接形态串）与
// S5 错凭据探针值（同口径 sensitiveTokens 数组）；命中即 FAIL。命中时不回显冒犯
// 内容（只打布尔/计数——红线自保）。
function assertOutputClean() {
  const leaked = emittedDetails.some((d) =>
    d.includes(UAT_CREDENTIAL) || d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t)));
  check('SEC', "输出自净：全部 detail 零凭据/token 值零 '/s/' 链接形态串（红线运行时自证）",
    !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}

const scenarios = [
  ['S1', s1Healthz],
  ['S2', s2MetricsAuthGate],
  ['S3', s3MetricsExposition],
  ['S4', s4Draining503],
  ['S5', s5AuditEvents],
  ['S6', s6ControlCharStrip],
];
// 调试场景过滤（PHASE08_ONLY=S1,S3——仅调试用；提交形态恒全场景开启）
const ONLY = process.env.PHASE08_ONLY?.split(',').map((s) => s.trim()) ?? null;
let failed = 0;
for (const [id, s] of scenarios) {
  if (ONLY !== null && !ONLY.includes(id)) continue;
  try {
    await s();
  } catch (e) {
    failed++;
    // WR-02：异常消息纳入 emittedDetails——assertOutputClean 自净断言面延伸到场景
    // 异常通道（此前该通道绕过扫描，startWesh 启动超时等消息可携敏感值静默破线）
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
