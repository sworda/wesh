// Phase 7 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch）。
// 覆盖 07-01..07-06 六 plan 服务端机制对真实二进制的全链断言（OPS-01/02/04/05/09/11、
// SEC-07、D-23）：S1 配置文件合并与优先级（TOML 铺底 401/503 生效、CLI 覆盖、未知键/
// 不存在文件 exit 2、权限警告不含值）、S2 unix socket 全链（启动行/socket 0660/残留
// 清理/TCP↔unix relay 转发后 WS echo）、S3 base-path 页面+WS 升级+share 交叉（307/
// 404/WS 双路径/share token×base-path 全链）、S4 auth-header 记录与 sanitize + XFF
// 换键（事件行 remote=XFF 链首 + remote_user=sanitize 值 + 未配置对照现状）、
// S5 stop-signal 宽限补 KILL（trap 忽略 TERM + 1s 后 KILL 退出 255；对照 TERM 即终结）、
// S6 降权 self（id -u == self + HOME/USER 身份环境一致）、S7 1001 关停序列（SIGTERM →
// close 1001 server_shutting_down → 退出 255）、S8 --open（headless 跳过不阻断 +
// fake xdg-open argv == rw 分享链接；真实弹浏览器 skip+reason 平台豁免）。
//
// 红线（phase06.mjs:11-13 纪律逐字沿用）：share token/凭据值只作断言材料，永不进入
// check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/退出码/文案常量
//（测试输出可能进 CI 日志，token 落盘即泄露样本）。S1 的 TOML 凭据值探针串与 share
// token 同口径入 sensitiveTokens 闭包数组，assertOutputClean 运行时自证零命中。
//
// 单次语义纪律（phase06.mjs:15-17 逐字沿用）：--exit-when-empty/1001 关停场景的服务端
// 进程退出是特性不是回归——child 'exit' 事件即断言通道（waitExit helper），spawn
// 实例 SIGKILL 收口仅用于未预期退出场景的清理。
//
// 运行：node web/uat/phase07.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
// 调试：PHASE07_ONLY=S1,S3 node web/uat/phase07.mjs（场景过滤，仅调试用——提交形态
// 恒为全场景开启）。
import { spawn } from 'node:child_process';
import http from 'node:http';
import crypto from 'node:crypto';
import net from 'node:net';
import { mkdtempSync, writeFileSync, chmodSync, statSync, existsSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
const SUBPROTOCOL = 'wesh.v1';

// UAT 专用凭据（phase03.mjs 同款；值不入任何输出——红线）；basicAuthHeader 仅作请求构造材料。
const UAT_CREDENTIAL = 'uat:uat-pass-x9';
const basicAuthHeader = () => 'Basic ' + Buffer.from(UAT_CREDENTIAL).toString('base64');
// S1 配置凭据按值构造（值同样只作请求构造材料——红线同口径）
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

// startWesh 解析 stdout 时把分享链接 token 留入本闭包数组（只作 assertOutputClean
// 断言材料）——红线：token 值永不进 check detail/控制台输出/汇总行。S1 的 TOML
// 凭据值探针串同口径入本数组（值同样永不进任何输出）。
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

// 启动 wesh 实例，返回 { port, scheme, shareRO, shareRW, unixPath, stderrText, stdoutText, kill, child }。
// opts.defaultListen（默认 true）：前置 --bind 127.0.0.1 --port 0（loopback 随机端口，
// 与用户服务零干扰）；false 时 argv 原样（S1 配置驱动 bind/port、S2 --socket 场景——
// --socket 与显式 --bind/--port 互斥，D-08 校验矩阵拒绝同给）。
// opts.unix（默认 false）：unix socket 启动行解析形态——'listening on unix://<path>' +
// 分享链接退化单行（D-12），无端口无 share token；TCP 形态照 phase06.mjs 三行解析
//（listening + share read-only 恒打印 + share read-write 仅 --writable）。
// opts.env（默认 process.env）：子进程环境（S8 --open 场景的 DISPLAY 增删/PATH 前置）。
// stderr 持续捕获（logEvent/警告行/panic 断言通道——S1e/S4/S8 消费）。
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
      // scheme 感知启动行解析（照 phase06.mjs 形态）；ro 行齐备后 50ms 落定窗吸纳 rw 行
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

// 拒绝路径 helper（phase03.mjs 形态）：进程预期 3s 内自行退出（启动校验拒绝路径不打印
// listening 行而是直接非零退出，startWesh 等端口必然超时，必须走 spawn-exit 形态）。
// S1c/S1d 配置加载失败 exit 2 场景消费；stderr 全量返回供类别/探针断言。
function spawnExpectExit(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, args, { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    child.stderr.on('data', (d) => { stderr += d; });
    const to = setTimeout(() => {
      child.kill('SIGKILL');
      reject(new Error(`wesh 未在 3s 内退出（拒绝路径应早退）: ${redactArgs(args)}`));
    }, 3000);
    child.on('exit', (code) => { clearTimeout(to); resolve({ code, stderr }); });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// 建立 WS 连接并完成 Hello 握手（可携 ticket、可定尺寸）；返回 { ws, frames }，frames 持续累积。
// opts.path（默认 '/ws'）：S3 base-path 场景的 '/wesh/ws' 形态。
// opts.headers（默认无）：S4 auth-header/XFF 场景的反代头注入——Node >= 22 原生
// WebSocket 第二参 { headers, protocols } 形态（本机探针实证：自定义头与 C1 控制
// 字符均可传输）。无 headers 时保持数组形态第二参（phase06.mjs 逐字）。
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

// waitExit：child 'exit' 事件决议 {code, signal}——S4/S5/S7 的 wesh 进程退出断言通道
//（本 phase 场景的服务端退出是特性不是回归）。恒带超时护栏：被测二进制挂死时护栏
// 到期 resolve(null) 由断言转 FAIL，而非无限等待（phase06.mjs 同款纪律）。
function waitExit(child, timeoutMs) {
  return new Promise((resolve) => {
    const to = setTimeout(() => resolve(null), timeoutMs);
    child.once('exit', (code, signal) => { clearTimeout(to); resolve({ code, signal }); });
  });
}

// 帧收集器 collectUntilClose(ws)（phase06.mjs 逐字）：换装 onmessage/onclose 为本收集器，
// close 到达时决议 {frames, close:{code,reason}}。S7 的 1001 关停序列断言通道。
function collectUntilClose(ws, timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    const frames = [];
    const to = setTimeout(() => reject(new Error(`collectUntilClose 超时：${timeoutMs}ms 未收到 close`)), timeoutMs);
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onclose = (ev) => { clearTimeout(to); resolve({ frames, close: { code: ev.code, reason: ev.reason } }); };
  });
}

// 手构 WS Upgrade 请求（phase06.mjs rawUpgrade 形态 + path 参数），resolve HTTP 状态码
//（101 = 升级成功）。S3 消费：base-path 下裸 /ws → 404（WS 层 bp 隔离）与 /wesh/ws → 101。
function rawUpgrade(port, path, headers) {
  return new Promise((resolve, reject) => {
    const key = crypto.randomBytes(16).toString('base64');
    const req = http.request({
      host: '127.0.0.1', port, path,
      headers: {
        Connection: 'Upgrade', Upgrade: 'websocket',
        'Sec-WebSocket-Key': key, 'Sec-WebSocket-Version': '13',
        ...headers,
      },
    });
    req.on('response', (res) => { res.resume(); resolve(res.statusCode); });
    req.on('upgrade', (_res, socket) => { socket.destroy(); resolve(101); });
    req.on('error', reject);
    req.end();
  });
}

// TCP↔unix relay 夹具（07-RESEARCH Pattern 7 本机探针实证形态）：Node 原生
// WebSocket/fetch 不能直连 unix socket（无全局 Agent、无 node:undici builtin、无裸
// undici 包——RESEARCH 探针三项实证），15 行管道转发后既有 WS 断言机制零改动复用。
// 场景结束必须 close()（relay 纪律——server 不关闭则进程不退，脚本悬挂）。
function startRelay(sockPath) {
  return new Promise((resolve, reject) => {
    const relay = net.createServer((c) => {
      const u = net.createConnection(sockPath);
      c.pipe(u).pipe(c);
    });
    relay.on('error', reject);
    relay.listen(0, '127.0.0.1', () => resolve({ port: relay.address().port, close: () => relay.close() }));
  });
}

// 取 frames 中首帧 WELCOME 的 JSON 载荷
const welcomeOf = (frames) => JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));

const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');

// INPUT 帧发送 helper（phase06.mjs 内联形态提为函数——S2/S6 echo 链路多次消费）
const sendInput = (ws, text) => ws.send(concat(new Uint8Array([INPUT]), enc.encode(text)));

// echo 链路轮询：frames[fromIdx..] 的 OUTPUT 文本含 marker 即 true（5s 默认护栏，
// phase06.mjs S5b/S6 同款 50ms 爬梯）
async function waitOutput(frames, fromIdx, marker, timeoutMs = 5000) {
  const t0 = Date.now();
  while (Date.now() - t0 < timeoutMs) {
    if (outputText(frames, fromIdx).includes(marker)) return true;
    await sleep(50);
  }
  return false;
}

// 正则锚定轮询（数字/路径提取用——S6 的 id -u 与 HOME/USER 断言）：返回首个 match 或 null
async function pollMatch(frames, fromIdx, regex, timeoutMs = 5000) {
  const t0 = Date.now();
  while (Date.now() - t0 < timeoutMs) {
    const m = regex.exec(outputText(frames, fromIdx));
    if (m) return m;
    await sleep(50);
  }
  return null;
}

// 临时目录夹具（S1 TOML/S2 socket 路径/S8 fake xdg-open）；调用方 finally 清理
const mkTmp = (tag) => mkdtempSync(join(tmpdir(), tag));
const rmTmp = (dir) => { try { rmSync(dir, { recursive: true, force: true }); } catch { /* 清理失败不致命 */ } };

// ---------- S1：配置文件合并与优先级（OPS-09：TOML 铺底生效 / CLI 覆盖 / 严格模式拒绝 / D-07 警告） ----------
async function s1ConfigMergeAndPrecedence() {
  console.log('S1: 配置文件（TOML 凭据 401 + max-clients=1 503 生效；CLI --max-clients 2 覆盖；未知键/不存在 exit 2 不含探针值；chmod 644 权限警告不含值）');
  // S1 全部 TOML 凭据值入 sensitiveTokens 同口径数组（值永不进任何 detail——红线）
  const CRED1 = 'cfg-op1:cfg-pass-one';
  const CRED2 = 'cfg-op2:cfg-pass-two';
  const PROBE = 'probe:probe-secret-zz9';
  const PERM = 'perm-op:perm-pass-w7';
  sensitiveTokens.push(CRED1, CRED2, PROBE, PERM);

  const tmp = mkTmp('wesh-p7-s1-');
  try {
    // ① TOML 铺底生效：port=0/bind/credential 两组/max-clients=1/command 全配置驱动
    //（spawn 不带 --bind/--port/-- —— 监听与命令全由配置给出，startWesh defaultListen:false）
    const cfgPath = join(tmp, 'wesh.toml');
    writeFileSync(cfgPath, [
      'port = 0',
      'bind = "127.0.0.1"',
      `credential = ["${CRED1}", "${CRED2}"]`,
      'max-clients = 1',
      'command = ["bash", "--norc", "--noprofile"]',
      '',
    ].join('\n'));
    chmodSync(cfgPath, 0o600); // 免 D-07 警告噪音（S1e 专测警告路径）
    const inst = await startWesh(['--config', cfgPath], { defaultListen: false });
    try {
      // 排序即解零 pacing（05-09 登记纪律）：先成功链路（A attach → B 503——成功
      // 认证 recordSuccess 清零节流），401 负面对照排最后（fail#1 +1s 窗口无后续消费者）
      const respA = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
        method: 'POST', headers: { Authorization: basicHeader(CRED1) },
      });
      const bodyA = respA.status === 200 ? await respA.json() : {};
      const a = await dialHello(inst.port, { ticket: bodyA.ticket });
      check('S1a', '配置凭据生效：A 经配置凭据 attach 成功（Basic → ticket → Welcome）',
        respA.status === 200 && a.ws.readyState === WebSocket.OPEN, `attach=${respA.status}`);
      // max-clients=1 配置生效：A 已占唯一槽位（dialHello 完成才计数），B 第二客户端 503
      const respB = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
        method: 'POST', headers: { Authorization: basicHeader(CRED2) },
      });
      check('S1b', '配置 max-clients=1 生效：第二客户端 POST /api/attach → 503',
        respB.status === 503, `status=${respB.status}`);
      await respB.text();
      // 配置凭据生效的负面对照：无凭据 GET / → 401（排最后——排序即解零 pacing）
      const respRoot = await fetch(`http://127.0.0.1:${inst.port}/`);
      check('S1c', '无凭据 GET / → 401（配置 credential 列表生效）',
        respRoot.status === 401, `status=${respRoot.status}`);
      await respRoot.text();
      a.ws.close();
      await waitClose(a.ws, 3000);
    } finally {
      inst.kill();
    }

    // ② CLI 覆盖：同配置 + --max-clients 2 → 两客户端并存（D-05 优先级链 flag > 配置）
    const inst2 = await startWesh(['--config', cfgPath, '--max-clients', '2'], { defaultListen: false });
    try {
      const respA = await fetch(`http://127.0.0.1:${inst2.port}/api/attach`, {
        method: 'POST', headers: { Authorization: basicHeader(CRED1) },
      });
      const bodyA = respA.status === 200 ? await respA.json() : {};
      const a = await dialHello(inst2.port, { ticket: bodyA.ticket });
      const respB = await fetch(`http://127.0.0.1:${inst2.port}/api/attach`, {
        method: 'POST', headers: { Authorization: basicHeader(CRED2) },
      });
      const bodyB = respB.status === 200 ? await respB.json() : {};
      const b = await dialHello(inst2.port, { ticket: bodyB.ticket });
      const wA = welcomeOf(a.frames);
      const wB = welcomeOf(b.frames);
      check('S1d', 'CLI 覆盖配置：--max-clients 2 下两客户端并存（双 Welcome）',
        respA.status === 200 && respB.status === 200
        && a.ws.readyState === WebSocket.OPEN && b.ws.readyState === WebSocket.OPEN,
        `attach=${respA.status}/${respB.status} mode=${wA.mode}/${wB.mode}`);
      a.ws.close();
      b.ws.close();
      await Promise.all([waitClose(a.ws, 3000), waitClose(b.ws, 3000)]);
    } finally {
      inst2.kill();
    }

    // ③ 未知键拒绝（D-06 严格模式）：exit 2 且 stderr 不含凭据探针值（值剥离红线——
    // go-toml 错误文本带源行上下文会回显值，configErr 只报类别+键名+行号）
    const badPath = join(tmp, 'bad.toml');
    writeFileSync(badPath, `credential = ["${PROBE}"]\nnonsense-key = true\n`);
    const bad = await spawnExpectExit(['--config', badPath]);
    check('S1e', '未知键 TOML → exit 2 且 stderr 含 unknown keys 类别且不含凭据探针值',
      bad.code === 2 && bad.stderr.includes('unknown keys') && !bad.stderr.includes('probe-secret-zz9'),
      `code=${bad.code} 类别=${bad.stderr.includes('unknown keys')} 探针缺席=${!bad.stderr.includes('probe-secret-zz9')}`);

    // ④ 不存在文件（D-06）：exit 2
    const missing = await spawnExpectExit(['--config', join(tmp, 'nonexistent.toml')]);
    check('S1f', '配置文件不存在 → exit 2', missing.code === 2, `code=${missing.code}`);

    // ⑤ D-07 权限警告：credential 键 + chmod 644 → stderr 警告行且不含凭据值（放行不阻断）
    const permPath = join(tmp, 'perm.toml');
    writeFileSync(permPath, [
      'port = 0',
      'bind = "127.0.0.1"',
      `credential = ["${PERM}"]`,
      'command = ["bash", "--norc", "--noprofile"]',
      '',
    ].join('\n'));
    chmodSync(permPath, 0o644);
    const inst3 = await startWesh(['--config', permPath], { defaultListen: false });
    try {
      // 警告在加载期打印（先于 listening 行）——startWesh resolve 时 stderr 已含警告行
      const warnLine = inst3.stderrText().includes('recommend chmod 600');
      const valueLeaked = inst3.stderrText().includes('perm-pass-w7');
      const respRoot = await fetch(`http://127.0.0.1:${inst3.port}/`);
      check('S1g', 'credential + chmod 644 → stderr 权限警告行且不含值，服务放行（401 生效）',
        warnLine && !valueLeaked && respRoot.status === 401,
        `警告行=${warnLine} 值缺席=${!valueLeaked} status=${respRoot.status}`);
      await respRoot.text();
    } finally {
      inst3.kill();
    }
  } finally {
    rmTmp(tmp);
  }
}

// ---------- S2：unix socket 全链（OPS-01：启动行/0660/残留清理 + relay 转发 WS echo） ----------
async function s2UnixSocketRelay() {
  console.log('S2: unix socket（预建垃圾文件残留清理 → 启动行 unix:// + socket 0660 + relay 转发 dialHello echo 全链 + 分享链接退化单行无 http://）');
  const tmp = mkTmp('wesh-p7-s2-');
  const sockPath = join(tmp, 'wesh.sock');
  let relay = null;
  try {
    // D-10 残留清理：listen 前 os.Remove——预建垃圾文件于 socket 路径，spawn 成功即证据
    writeFileSync(sockPath, 'stale-garbage-not-a-socket');
    const inst = await startWesh(['--socket', sockPath, '--writable', '--', 'bash', '--norc', '--noprofile'],
      { defaultListen: false, unix: true });
    try {
      check('S2a', '残留垃圾文件预置下 spawn 成功且启动行含 unix://（D-10 清理 + D-12 打印）',
        inst.unixPath !== null && inst.unixPath.startsWith('unix://'),
        `启动行=${inst.unixPath !== null}`);
      const st = statSync(sockPath);
      check('S2b', 'socket 文件存在且为 socket 类型且权限恰为 0660（显式 Chmod 不靠 umask）',
        st.isSocket() && (st.mode & 0o777) === 0o660,
        `socket=${st.isSocket()} mode=${(st.mode & 0o777).toString(8)}`);
      check('S2c', 'stdout 无 http:// 分享链接行且退化单行存在（D-12——不拼误导性 TCP 链接）',
        !inst.stdoutText().includes('http://') && inst.stdoutText().includes('unavailable on unix socket'),
        `无http链接=${!inst.stdoutText().includes('http://')} 退化行=${inst.stdoutText().includes('unavailable on unix socket')}`);

      // relay 夹具：原生 WebSocket 不能直连 unix socket——TCP 管道转发后既有断言零改动
      relay = await startRelay(sockPath);
      const c = await dialHello(relay.port, {});
      check('S2d', 'relay 转发后 dialHello 完成（unix socket 上 WS 握手全链）',
        c.ws.readyState === WebSocket.OPEN, 'Welcome 到达');
      // echo 全链：INPUT 唯一标记回读 OUTPUT（--writable 实例）
      const MARK = 'UAT_S2_RELAY_q8w2e';
      const base = c.frames.length;
      sendInput(c.ws, `echo ${MARK}\r`);
      const echoed = await waitOutput(c.frames, base, MARK);
      check('S2e', 'relay 转发后 echo 全链（INPUT 标记回读 OUTPUT 含标记）', echoed, `echo=${echoed}`);
      c.ws.close();
      await waitClose(c.ws, 3000);
    } finally {
      inst.kill();
    }
  } finally {
    if (relay !== null) relay.close(); // relay 纪律：不关闭则脚本悬挂
    rmTmp(tmp);
  }
}

// ---------- S3：base-path 页面 + WS 升级 + share 交叉（OPS-02：307/404/WS/share token 全链） ----------
async function s3BasePathCross() {
  console.log('S3: base-path（/wesh→307 + /wesh/→200 + /→404 + WS 双路径隔离 + share 行含前缀 + share token×base-path 全链 attach）');
  const inst = await startWesh(['--base-path', '/wesh', '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    // 裸 /wesh → 307 保方法重定向（mux matchOrRedirect 免费形态，GOROOT 实证）
    const respBare = await fetch(`http://127.0.0.1:${inst.port}/wesh`, { redirect: 'manual' });
    const location = respBare.headers.get('location') ?? '';
    check('S3a', 'GET /wesh（裸）→ 307 且 Location 含 /wesh/（尾斜杠规范化）',
      respBare.status === 307 && location.includes('/wesh/'),
      `status=${respBare.status} Location含前缀=${location.includes('/wesh/')}`);
    await respBare.text();
    const respPage = await fetch(`http://127.0.0.1:${inst.port}/wesh/`);
    const pageText = await respPage.text();
    check('S3b', 'GET /wesh/ → 200 终端页（HTML 伺服经 StripPrefix）',
      respPage.status === 200 && pageText.includes('<html'),
      `status=${respPage.status} html=${pageText.includes('<html')}`);
    const respRoot = await fetch(`http://127.0.0.1:${inst.port}/`);
    check('S3c', 'GET / → 404（base 外路径不伺服——bp 隔离）', respRoot.status === 404, `status=${respRoot.status}`);
    await respRoot.text();
    // WS 层双路径：裸 /ws → 404（未注册），/wesh/ws → 101 升级
    const bareWs = await rawUpgrade(inst.port, '/ws', { 'Sec-WebSocket-Protocol': SUBPROTOCOL });
    const bpWs = await rawUpgrade(inst.port, '/wesh/ws', { 'Sec-WebSocket-Protocol': SUBPROTOCOL });
    check('S3d', 'WS 层 bp 隔离：裸 /ws → 404 且 /wesh/ws → 101 升级',
      bareWs === 404 && bpWs === 101, `bare=${bareWs} bp=${bpWs}`);
    const c = await dialHello(inst.port, { path: '/wesh/ws' });
    check('S3e', 'WS /wesh/ws dialHello 完成（base-path 下握手全链）',
      c.ws.readyState === WebSocket.OPEN, 'Welcome 到达');
    c.ws.close();
    await waitClose(c.ws, 3000);
    // 启动打印 share 行含 base-path 前缀（D-14 拼串单一事实源——值只作断言材料）
    check('S3f', '启动打印 share 行含 /wesh/s/ 前缀（ro/rw 两行）',
      inst.shareRO !== null && inst.shareRW !== null
      && inst.shareRO.includes('/wesh/s/') && inst.shareRW.includes('/wesh/s/'),
      `ro含前缀=${inst.shareRO?.includes('/wesh/s/') ?? false} rw含前缀=${inst.shareRW?.includes('/wesh/s/') ?? false}`);
    // share token × base-path 交叉全链：GET /wesh/s/{token}/ → 200 → POST 携 token →
    // ticket → Hello 经 /wesh/ws → Welcome（RESEARCH Pitfall 3 的 401 回归链防线）
    const shareUrlPath = new URL(inst.shareRW).pathname;
    const respShare = await fetch(`http://127.0.0.1:${inst.port}${shareUrlPath}`);
    check('S3g', 'GET /wesh/s/{token}/ → 200（share 页经 base-path 可达）',
      respShare.status === 200, `status=${respShare.status}`);
    await respShare.text();
    const respAttach = await fetch(`http://127.0.0.1:${inst.port}/wesh/api/attach`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tokenFromUrl(inst.shareRW) }),
    });
    const bodyAttach = respAttach.status === 200 ? await respAttach.json() : {};
    const c2 = await dialHello(inst.port, { path: '/wesh/ws', ticket: bodyAttach.ticket });
    const w2 = welcomeOf(c2.frames);
    check('S3h', 'share token×base-path 交叉全链：POST 携 token → ticket → Hello 经 /wesh/ws → Welcome（rw）',
      respAttach.status === 200 && c2.ws.readyState === WebSocket.OPEN && w2.mode === 'rw',
      `attach=${respAttach.status} mode=${w2.mode}`);
    c2.ws.close();
    await waitClose(c2.ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S4：auth-header 记录与 sanitize + XFF 换键（SEC-07：D-15..D-20 真实二进制全链） ----------
// 事件行触发形态：--exit-when-empty 实例 attach+close → 注册表空 → exit_when_empty
// logEvent 携最后离开者 remote/remote_user（clients.go maybeExitWhenEmptyLocked——
// attach+close 链路的确定性即时事件）；进程随后 HUP 收口退出 255（顺带锁定）。
async function s4AuthHeaderXff() {
  console.log('S4: auth-header/XFF（remote=XFF 链首 + remote_user=alice；NEL 控制字符剥离 alice 保留；对照组现状 remote=127.0.0.1 系无 remote_user 键）');
  // ① trust 开启：X-Remote-User + XFF 链 → attach+close → 事件行 remote=链首 + remote_user
  const instA = await startWesh(['--auth-header', 'X-Remote-User', '--exit-when-empty', '--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(instA.port, {
      headers: { 'X-Remote-User': 'alice', 'X-Forwarded-For': '198.51.100.7, 10.0.0.2' },
    });
    check('S4a', '携 X-Remote-User + XFF 完成 attach（trust 开启，握手全链）',
      c.ws.readyState === WebSocket.OPEN, 'Welcome 到达');
    const exitP = waitExit(instA.child, 10000);
    c.ws.close();
    const proc = await exitP;
    // remote 字段 = XFF 链首（D-20：多值取首个 + TrimSpace）；remote_user = sanitize 后头值
    const okRemote = instA.stderrText().includes('remote=198.51.100.7 ');
    const okUser = instA.stderrText().includes('remote_user=alice');
    const okReason = instA.stderrText().includes('reason=exit_when_empty');
    check('S4b', 'attach+close 后事件行：remote=XFF 链首 且 remote_user=alice（reason=exit_when_empty）',
      proc !== null && proc.code === 255 && okRemote && okUser && okReason,
      `code=${proc?.code ?? '（未到）'} remote链首=${okRemote} remote_user=${okUser} reason=${okReason}`);
  } finally {
    instA.kill();
  }

  // ② sanitize：头值嵌控制字符 → 剥离后 alice 保留且控制字符不出日志（D-19）
  // plan 字面探针 'alice\r\nFAKE' 的 C0 \r\n 在真实 HTTP 头值不可传输（客户端栈即拒——
  // httpguts/undici 双侧同拒）；C1 NEL（U+0085）是可传输等价物（本机探针实证），Go 侧
  // TestSanitizeRemoteUser 已表驱动覆盖 C0/DEL 剥离。
  // 线形构造（本机两探针实证）：undici 头值按 latin1 编码上线——直接发 JS 'ali\u0085ce'
  // 线上为单字节 0x85（Go 侧非法 UTF-8 → U+FFFD，复现不了剥离路径，S4c 初跑实测命中）；
  // 发 JS 'ali\u00C2\u0085ce'（U+00C2 U+0085 两码点）上线字节 = 0xC2 0x85 = UTF-8 客户端
  //（Go http/curl）发送 'ali\u0085ce' 的等价线形——Go 解码得 U+0085 NEL，按 D-19 剥离 → 'alice'
  const instB = await startWesh(['--auth-header', 'X-Remote-User', '--exit-when-empty', '--', 'bash', '--norc', '--noprofile']);
  try {
    const NEL_WIRE = 'ali\u00C2\u0085ce'; // 线形等价物：undici latin1 上线后字节 = UTF-8 编码的 ali<NEL>ce（见上注释块）
    const c = await dialHello(instB.port, { headers: { 'X-Remote-User': NEL_WIRE } });
    const exitP = waitExit(instB.child, 10000);
    c.ws.close();
    const proc = await exitP;
    const aliceKept = instB.stderrText().includes(' remote_user=alice\n');
    const nelLeaked = instB.stderrText().includes('\u0085');
    check('S4c', 'sanitize：NEL 控制字符剥离后 alice 保留且控制字符零泄漏（D-19 日志注入防线）',
      proc !== null && proc.code === 255 && aliceKept && !nelLeaked,
      `code=${proc?.code ?? '（未到）'} alice保留=${aliceKept} 控制字符缺席=${!nelLeaked}`);
  } finally {
    instB.kill();
  }

  // ③ 对照组：无 --auth-header → 同请求日志行为现状（remote=TCP 对端 host:port，
  // XFF 完全忽略，无 remote_user 键——D-20 单一信任闸零双轨）
  const instC = await startWesh(['--exit-when-empty', '--', 'bash', '--norc', '--noprofile']);
  try {
    const c = await dialHello(instC.port, {
      headers: { 'X-Remote-User': 'alice', 'X-Forwarded-For': '198.51.100.7' },
    });
    const exitP = waitExit(instC.child, 10000);
    c.ws.close();
    const proc = await exitP;
    const loopbackRemote = /remote=127\.0\.0\.1:\d+/.test(instC.stderrText());
    const noUserKey = !instC.stderrText().includes('remote_user=');
    const xffIgnored = !instC.stderrText().includes('198.51.100.7');
    check('S4d', '对照组无 --auth-header：remote=127.0.0.1 系（host:port 现状）且无 remote_user 键且 XFF 忽略',
      proc !== null && proc.code === 255 && loopbackRemote && noUserKey && xffIgnored,
      `code=${proc?.code ?? '（未到）'} loopback=${loopbackRemote} 无remote_user键=${noUserKey} XFF忽略=${xffIgnored}`);
  } finally {
    instC.kill();
  }
}

// 输出自净断言（WR-02/review #7 形态——红线由注释纪律升级为运行时自证）：遍历全部已发
// detail，断言不含 UAT_CREDENTIAL 值、任一 share token 值（含 '/s/' 链接形态串）与
// S1 TOML 凭据探针值（同口径 sensitiveTokens 数组）；命中即 FAIL。命中时不回显冒犯
// 内容（只打布尔/计数——红线自保）。
function assertOutputClean() {
  const leaked = emittedDetails.some((d) =>
    d.includes(UAT_CREDENTIAL) || d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t)));
  check('SEC', "输出自净：全部 detail 零凭据/token 值零 '/s/' 链接形态串（红线运行时自证）",
    !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}

const scenarios = [
  ['S1', s1ConfigMergeAndPrecedence],
  ['S2', s2UnixSocketRelay],
  ['S3', s3BasePathCross],
  ['S4', s4AuthHeaderXff],
];
// 调试场景过滤（PHASE07_ONLY=S1,S3——仅调试用；提交形态恒全场景开启）
const ONLY = process.env.PHASE07_ONLY?.split(',').map((s) => s.trim()) ?? null;
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
