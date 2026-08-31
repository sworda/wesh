// Phase 9 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/http）。
// 覆盖 09-04 自定义首页 --index 全行为面对真实二进制的全链断言（OPS-03、
// D-05..D-08、09-UI-SPEC §Custom Index Contract 1-7 逐字边界）：
//   S1 启动校验矩阵（独立 spawn 期望 exit 2 不启动：--index 不存在文件/目录
//      （非常规）/TOML index-max-size 调小（64）+ 超限探针文件；三行 stderr 均
//      含路径与类别且零内容探针——D-07/D-08 启动面红线反断言）；
//   S2 给页（spawn --index probe.html 常态实例：GET /（空路径回落）与
//      /index.html 与 /s/{ro-token}/（启动行解析取分享链接）三通道自定义字节
//      byte-identity（D-05 整页替换/D-06 全通道统一）；GET /x.css → 404（相对
//      资源契约语义）；安全头同源——CSP 含 connect-src 'self' 且六安全头与
//      内建页响应逐头同值（契约行 5）；
//   S3 gzip/Vary（Accept-Encoding 显式含 gzip → Content-Encoding: gzip 且
//      gunzip 后 byte-identity；无 Accept-Encoding → 明文 byte-identity；两态
//      Vary: Accept-Encoding 恒在 + Content-Type text/html; charset=utf-8——
//      契约行 4/6）；
//   S4 认证面照旧（无认证模式 POST /api/attach → 404 探测信号 + WS Hello→
//      Welcome 全链（D-05「WS 端点照旧暴露」+ 契约行 7）；凭据模式 GET /
//      带凭据 → 200 自定义字节、无凭据 → 401——认证闸在装饰链外层）；
//   S5 0 字节与 base-path（--index 空文件 → 200 空 body（D-07 拒绝列表不含
//      空文件）；--index + --base-path /wesh → GET /wesh/ 给自定义字节、GET /
//      → 404 根无挂载（契约行 6））；
//   S6 配置通道（TOML index 键（无 CLI）生效；配置 index + CLI --index 另文件
//      → CLI 覆盖生效（D-07 TOML 同名键铺底、flag > 配置优先级链））。
//
// 红线（phase06.mjs:11-13/phase07.mjs:14-17 纪律逐字沿用）：share token/凭据值/
// 探针串只作断言材料，永不进入 check detail 或任何控制台输出——detail 只打印
// 状态码/布尔/形状/退出码/文案常量（测试输出可能进 CI 日志，token 落盘即泄露
// 样本）。探针内容串（CIDX-PROBE-*）与 UAT 凭据值同口径入 sensitiveTokens
// 闭包数组，assertOutputClean 运行时自证零命中。
//
// 本 phase 无平台豁免面（协议层全链 headless 可断言——CODEBUDDY.md 分层策略
// 层 1）；skip helper 按三件套形态保留但零调用。
//
// 运行：node web/uat/phase09.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
// 调试：PHASE09_ONLY=S1,S3 node web/uat/phase09.mjs（场景过滤，仅调试用——提交
// 形态恒为全场景开启）。
import { spawn } from 'node:child_process';
import http from 'node:http';
import crypto from 'node:crypto';
import { mkdtempSync, writeFileSync, mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { gunzipSync } from 'node:zlib';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐——本脚本仅握手机两帧）
const HELLO = 0x48, WELCOME = 0x57;
const SUBPROTOCOL = 'wesh.v1';

// UAT 专用凭据（phase03/07.mjs 同款；值不入任何输出——红线）；basicAuthHeader
// 仅作请求构造材料。
const UAT_CREDENTIAL = 'uat:uat-pass-x9';
const basicAuthHeader = () => 'Basic ' + Buffer.from(UAT_CREDENTIAL).toString('base64');

const enc = new TextEncoder();
const concat = (...parts) => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) { out.set(p, off); off += p.length; }
  return out;
};
// Hello 载荷 {version,cols,rows}（无认证模式无 ticket 键——omitempty 对称形态）
const helloFrame = ({ version = SUBPROTOCOL, cols = 80, rows = 24 } = {}) =>
  concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({ version, cols, rows })));

const results = [];
// 全部已发 detail 收集（assertOutputClean 遍历材料——运行时自净断言）
const emittedDetails = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  emittedDetails.push(String(detail));
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
// 平台豁免记录形态：不计失败（phase07.mjs 三件套形态保留；本 phase 无平台
// 豁免面——协议层全链 headless 可断言，零调用）
const skip = (id, name, reason) => {
  results.push({ id, name, ok: null });
  emittedDetails.push(String(reason));
  console.log(`  SKIP  ${id} ${name} — ${reason}`);
};

// startWesh 解析 stdout 时把分享链接 token 留入本闭包数组（只作 assertOutputClean
// 断言材料）——红线：token 值永不进 check detail/控制台输出/汇总行。探针内容串
// 同口径入本数组（值同样永不进任何输出）。
const sensitiveTokens = [];

// 内容探针前缀（探针文件内容含唯一探针串 CIDX-PROBE-<随机>——S1 三拒绝场景
// stderr 零内容探针反断言材料源头；探针值只进 sensitiveTokens，永不进任何输出）
const PROBE_PREFIX = 'CIDX-PROBE-';

// 分享链接 URL → token（/s/{token}/ 路径段；值只作断言材料——红线）
const tokenFromUrl = (url) => /\/s\/([^/]+)\//.exec(url)[1];

// 启动超时 reject 消息脱敏（phase07.mjs 形态）：--credential 后随值（空格与 =
// 两形态）替换为 <redacted>；场景异常通道（尾部 catch 原样打印 e.message）经
// emittedDetails 进 assertOutputClean 扫描面，argv 原样回显会把凭据值明文送进
// 控制台/CI 日志。
const redactArgs = (args) => args.map((a, i) => {
  if (a.startsWith('--credential=')) return '--credential=<redacted>';
  if (i > 0 && args[i - 1] === '--credential') return '<redacted>';
  return a;
}).join(' ');

// 启动 wesh 实例，返回 { port, shareRO, shareRW, stderrText, stdoutText, kill, child }
// （phase07.mjs 夹具形态——TCP 场景专用：unix 分支无本 phase 消费场景，省略）。
// 前置 --bind 127.0.0.1 --port 0（loopback 随机端口，与用户服务零干扰）；
// opts.defaultListen（默认 true）false 时 argv 原样（S6 配置驱动 bind/port 场景
// ——监听与命令全由 TOML 给出，phase07 S1 先例形态）。
function startWesh(args, { defaultListen = true } = {}) {
  return new Promise((resolve, reject) => {
    const argv = defaultListen ? ['--bind', '127.0.0.1', '--port', '0', ...args] : args;
    const child = spawn(WESH, argv, { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    let stdoutBuf = '';
    let settling = false;
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${redactArgs(argv)}; stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      stdoutBuf += d.toString();
      if (settling) return;
      // 启动行解析（照 phase07.mjs 形态）；ro 行齐备后 50ms 落定窗吸纳 rw 行
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
          resolve({ port: Number(m[2]), shareRO, shareRW, stderrText: () => stderr, stdoutText: () => stdoutBuf, kill: () => child.kill('SIGKILL'), child });
        }, 50);
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

// 拒绝路径 helper（phase03/07.mjs 形态）：进程预期 3s 内自行退出（启动校验拒绝
// 路径不打印 listening 行而是直接非零退出，startWesh 等端口必然超时，必须走
// spawn-exit 形态）。S1 三拒绝场景消费；stderr 全量返回供类别/路径/探针断言
//（不外打印——红线）。
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

// rawFetch（node:http 原始请求——给页断言通道）：不注入 Accept-Encoding、不
// 透明解压（undici fetch 会自动加 accept-encoding 且按 Content-Encoding 透传
// 解压，明文伺服态结构性不可观测——09-04 Go transport 自动 gzip 同款语义适配）；
// 返回 { status, headers, body:Buffer }；5s 超时护栏防挂死。
function rawFetch(port, reqPath, { method = 'GET', headers = {} } = {}) {
  return new Promise((resolve, reject) => {
    const req = http.request({ host: '127.0.0.1', port, path: reqPath, method, headers }, (res) => {
      const chunks = [];
      res.on('data', (c) => chunks.push(c));
      res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks) }));
    });
    req.setTimeout(5000, () => req.destroy(new Error(`rawFetch 超时: ${method} ${reqPath}`)));
    req.on('error', reject);
    req.end();
  });
}

// 建立 WS 连接并完成 Hello 握手（phase07.mjs dialHello 形态收窄为无认证无参
// 形状——本脚本唯一消费点 S4b 无 ticket/自定义头/路径需求）；返回
// { ws, frames }，frames 持续累积。
function dialHello(port) {
  return new Promise((resolve, reject) => {
    const url = `ws://127.0.0.1:${port}/ws`;
    const ws = new WebSocket(url, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame({}));
    ws.onerror = () => reject(new Error('WS 连接失败'));
    // 总超时 watchdog：被测二进制挂死（Welcome 永不到达）时 10s 拒绝而非永久悬挂
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

// 临时目录夹具（探针文件/TOML）；调用方 finally 清理
const mkTmp = (tag) => mkdtempSync(join(tmpdir(), tag));
const rmTmp = (dir) => { try { rmSync(dir, { recursive: true, force: true }); } catch { /* 清理失败不致命 */ } };

// 探针 HTML 夹具：生成唯一探针串（sensitiveTokens 登记——内容红线反断言材料）
// 并落盘，返回 { path, bytes }；bytes 为 byte-identity 断言材料——长度/布尔之外
// 的形态永不入任何输出。文件 ~129 字节（> S1c 的 64 上限——超限触发形态）。
const writeProbe = (tmp, name) => {
  const mark = PROBE_PREFIX + crypto.randomBytes(6).toString('hex');
  sensitiveTokens.push(mark);
  const bytes = Buffer.from(
    `<!doctype html><html><head><title>wesh uat</title></head><body>${mark} custom index page ${mark}</body></html>`);
  writeFileSync(join(tmp, name), bytes);
  return { path: join(tmp, name), bytes };
};

// ---------- S1：启动校验矩阵（D-07/D-08：三拒绝 exit 2 + 错误行含路径与类别 + 零内容探针） ----------
async function s1StartupValidationMatrix() {
  console.log('S1: 启动校验矩阵（--index 不存在/目录/超限三拒绝 exit 2 不启动；stderr 含路径与类别且零内容探针——启动面红线反断言）');
  const tmp = mkTmp('wesh-p9-s1-');
  try {
    // 探针夹具先行：S1c 超限场景的 >64 字节文件；其内容探针前缀即三行 stderr
    // 共同的反断言材料（探针串只存 sensitiveTokens 闭包——红线）
    const big = writeProbe(tmp, 'big.html');

    // ① 不存在文件：validateStartup stat 预检第一类别（exit 2 fail-fast）
    const missingPath = join(tmp, 'nope.html');
    const missing = await spawnExpectExit(['--index', missingPath, '--', 'bash', '--norc', '--noprofile']);
    check('S1a', '--index 不存在文件 → exit 2 + stderr 含路径与不存在类别 + 零内容探针',
      missing.code === 2 && missing.stderr.includes(missingPath)
      && missing.stderr.includes('file does not exist') && !missing.stderr.includes(PROBE_PREFIX),
      `code=${missing.code} 路径=${missing.stderr.includes(missingPath)} 类别=${missing.stderr.includes('file does not exist')} 探针缺席=${!missing.stderr.includes(PROBE_PREFIX)}`);

    // ② 目录（非常规文件类别——设备/socket 等同档拒绝，目录为可移植触发形态）
    const dirPath = join(tmp, 'adir');
    mkdirSync(dirPath);
    const dir = await spawnExpectExit(['--index', dirPath, '--', 'bash', '--norc', '--noprofile']);
    check('S1b', '--index 目录 → exit 2 + stderr 含路径与非常规类别 + 零内容探针',
      dir.code === 2 && dir.stderr.includes(dirPath)
      && dir.stderr.includes('not a regular file') && !dir.stderr.includes(PROBE_PREFIX),
      `code=${dir.code} 路径=${dir.stderr.includes(dirPath)} 类别=${dir.stderr.includes('not a regular file')} 探针缺席=${!dir.stderr.includes(PROBE_PREFIX)}`);

    // ③ TOML index-max-size 调小（64，D-08 纯配置键无 CLI flag）+ --index 129
    // 字节探针文件 → 超限类别（含上限数值）；命令经 TOML 给出（S1c 触发形态即
    // 配置通道——plan「index-max-size 调小触发超限拒绝场景」字面形态）
    const cfgPath = join(tmp, 'small.toml');
    writeFileSync(cfgPath, [
      'bind = "127.0.0.1"',
      'port = 0',
      'index-max-size = 64',
      'command = ["bash", "--norc", "--noprofile"]',
      '',
    ].join('\n'));
    const exceeded = await spawnExpectExit(['--config', cfgPath, '--index', big.path]);
    check('S1c', 'TOML index-max-size 调小 + --index 超限文件 → exit 2 + 超限类别（含上限数值）+ 零内容探针',
      exceeded.code === 2 && exceeded.stderr.includes(big.path)
      && exceeded.stderr.includes('exceeds index-max-size') && exceeded.stderr.includes('(64 bytes)')
      && !exceeded.stderr.includes(PROBE_PREFIX),
      `code=${exceeded.code} 路径=${exceeded.stderr.includes(big.path)} 类别=${exceeded.stderr.includes('exceeds index-max-size')} 探针缺席=${!exceeded.stderr.includes(PROBE_PREFIX)}`);
  } finally {
    rmTmp(tmp);
  }
}

// ---------- S2：给页（D-05/D-06：三通道 byte-identity + 相对资源 404 + 安全头同源） ----------
async function s2PageServing() {
  console.log('S2: 给页（GET / 与 /index.html 与 /s/{ro-token}/ 三通道自定义字节 byte-identity + /x.css 404 + 安全头同源（CSP 与内建页逐头同值））');
  const tmp = mkTmp('wesh-p9-s2-');
  try {
    const probe = writeProbe(tmp, 'probe.html');
    const inst = await startWesh(['--index', probe.path, '--', 'bash', '--norc', '--noprofile']);
    try {
      const r1 = await rawFetch(inst.port, '/');
      check('S2a', 'GET / → 200 且 body 与探针文件字节 byte-identity（空路径回落，D-05 整页替换）',
        r1.status === 200 && r1.body.equals(probe.bytes),
        `status=${r1.status} bytes相等=${r1.body.equals(probe.bytes)}`);
      const r2 = await rawFetch(inst.port, '/index.html');
      check('S2b', 'GET /index.html → 200 同字节（显式 index.html 路径——契约行 1 唯一替换点）',
        r2.status === 200 && r2.body.equals(probe.bytes),
        `status=${r2.status} bytes相等=${r2.body.equals(probe.bytes)}`);
      // D-06 全通道统一：share 链接通道（启动行解析取 ro 链接——URL 只作请求
      // 构造材料，detail 只报状态码/布尔，红线）
      const sharePath = new URL(inst.shareRO).pathname;
      const r3 = await rawFetch(inst.port, sharePath);
      check('S2c', 'GET /s/{ro-token}/ → 200 同字节（D-06 全通道统一——装饰层在 sharePage 委托上游）',
        r3.status === 200 && r3.body.equals(probe.bytes),
        `status=${r3.status} bytes相等=${r3.body.equals(probe.bytes)}`);
      const r4 = await rawFetch(inst.port, '/x.css');
      check('S2d', 'GET /x.css → 404（相对资源契约语义——index.html 之外路径照旧 FileServerFS）',
        r4.status === 404, `status=${r4.status}`);
      // S2e 安全头同源（契约行 5）：CSP 含 connect-src 'self'（自定义页自实现
      // 终端可回连 /ws——D-05 承诺在 CSP 下成立）且六安全头与内建页响应逐头
      // 同值（securityHeaders 最外层装配不区分内建/自定义页——现状同源）
      const builtin = await startWesh(['--', 'bash', '--norc', '--noprofile']);
      try {
        const rb = await rawFetch(builtin.port, '/');
        const csp = r1.headers['content-security-policy'] ?? '';
        const cspOk = csp.includes("connect-src 'self'");
        const headerNames = ['content-security-policy', 'x-frame-options', 'x-content-type-options',
          'referrer-policy', 'cross-origin-opener-policy', 'cross-origin-resource-policy'];
        const allPresent = headerNames.every((n) => (r1.headers[n] ?? '') !== '');
        const allEqual = headerNames.every((n) => (r1.headers[n] ?? '') === (rb.headers[n] ?? ''));
        check('S2e', "GET / 安全头同源：CSP 含 connect-src 'self' 且六安全头与内建页响应逐头同值（契约行 5）",
          cspOk && allPresent && allEqual,
          `CSP含connect-src=${cspOk} 全在场=${allPresent} 六头同值=${allEqual}`);
      } finally {
        builtin.kill();
      }
    } finally {
      inst.kill();
    }
  } finally {
    rmTmp(tmp);
  }
}

// ---------- S3：gzip/Vary（契约行 4/6：预压双态 + Vary 恒发 + Content-Type） ----------
async function s3GzipVary() {
  console.log('S3: gzip/Vary（Accept-Encoding 显式 gzip → Content-Encoding: gzip + gunzip byte-identity；无 Accept-Encoding → 明文 byte-identity；两态 Vary 恒在 + Content-Type）');
  const tmp = mkTmp('wesh-p9-s3-');
  try {
    const probe = writeProbe(tmp, 'probe.html');
    const inst = await startWesh(['--index', probe.path, '--', 'bash', '--norc', '--noprofile']);
    try {
      // ① gzip 显式编码 → 装饰期预压体 + Content-Encoding: gzip（rawFetch 不透
      // 明解压——gzip 伺服态可观测）；gunzip 后 byte-identity（§4 预压定稿）
      const rg = await rawFetch(inst.port, '/', { headers: { 'Accept-Encoding': 'gzip' } });
      let gunzipped = null;
      try { gunzipped = gunzipSync(rg.body); } catch { /* 非法 gzip 流由断言布尔收口 */ }
      check('S3a', 'Accept-Encoding: gzip → 200 + Content-Encoding: gzip + gunzip 后 byte-identity + Vary（§4 预压定稿）',
        rg.status === 200 && rg.headers['content-encoding'] === 'gzip'
        && gunzipped !== null && gunzipped.equals(probe.bytes)
        && rg.headers.vary === 'Accept-Encoding',
        `status=${rg.status} 编码=${rg.headers['content-encoding'] ?? '（无）'} 解压相等=${gunzipped !== null && gunzipped.equals(probe.bytes)} Vary=${rg.headers.vary === 'Accept-Encoding'}`);
      // ② 无 Accept-Encoding → 明文 byte-identity（零 Content-Encoding）+ Vary
      // 恒在 + Content-Type text/html; charset=utf-8（契约行 6）
      const rp = await rawFetch(inst.port, '/');
      check('S3b', '无 Accept-Encoding → 200 明文 byte-identity + 零 Content-Encoding + Vary + Content-Type text/html; charset=utf-8',
        rp.status === 200 && rp.body.equals(probe.bytes)
        && (rp.headers['content-encoding'] ?? '') === ''
        && rp.headers.vary === 'Accept-Encoding'
        && rp.headers['content-type'] === 'text/html; charset=utf-8',
        `status=${rp.status} 明文相等=${rp.body.equals(probe.bytes)} 零编码=${(rp.headers['content-encoding'] ?? '') === ''} Vary=${rp.headers.vary === 'Accept-Encoding'} CT=${rp.headers['content-type'] === 'text/html; charset=utf-8'}`);
    } finally {
      inst.kill();
    }
  } finally {
    rmTmp(tmp);
  }
}

// ---------- S4：认证面照旧（D-05「WS 端点照旧暴露」+ 契约行 7：/api/attach 与 /ws 不受 --index 影响） ----------
async function s4AuthSurface() {
  console.log('S4: 认证面照旧（无认证 POST /api/attach → 404 + WS Hello→Welcome 全链；凭据模式带凭据 → 200 自定义字节、无凭据 → 401）');
  const tmp = mkTmp('wesh-p9-s4-');
  try {
    const probe = writeProbe(tmp, 'probe.html');
    // ① 无认证模式：/api/attach 404 探测信号照旧 + WS 握手全链（T-09-04d 同面）
    const inst = await startWesh(['--index', probe.path, '--', 'bash', '--norc', '--noprofile']);
    try {
      const ra = await rawFetch(inst.port, '/api/attach', { method: 'POST' });
      check('S4a', '无认证模式 POST /api/attach → 404（探测信号照旧——契约行 7 /api/attach 不受 --index 影响）',
        ra.status === 404, `status=${ra.status}`);
      const c = await dialHello(inst.port);
      check('S4b', 'WS /ws Hello→Welcome 全链（D-05：WS 端点照旧暴露——自定义页部署下终端协议面不变）',
        c.ws.readyState === WebSocket.OPEN, 'Welcome 到达');
      c.ws.close();
      await waitClose(c.ws, 3000);
    } finally {
      inst.kill();
    }

    // ② 凭据模式：认证闸在装饰链外层——带凭据 GET / → 200 自定义字节、无凭据
    // → 401（排序即解零 pacing（05-09 登记纪律）：成功链路在前，401 负面对照
    // 排最后——fail#1 +1s 窗口无后续消费者）
    const inst2 = await startWesh(['--credential', UAT_CREDENTIAL, '--index', probe.path, '--', 'bash', '--norc', '--noprofile']);
    try {
      const ok = await rawFetch(inst2.port, '/', { headers: { Authorization: basicAuthHeader() } });
      const denied = await rawFetch(inst2.port, '/');
      check('S4c', '凭据模式：带凭据 GET / → 200 自定义字节且无凭据 → 401（认证闸在装饰链外层）',
        ok.status === 200 && ok.body.equals(probe.bytes) && denied.status === 401,
        `带凭据=${ok.status} bytes相等=${ok.body.equals(probe.bytes)} 无凭据=${denied.status}`);
    } finally {
      inst2.kill();
    }
  } finally {
    rmTmp(tmp);
  }
}

// ---------- S5：0 字节与 base-path（D-07 空文件合法 + 契约行 6 bp 组合） ----------
async function s5EmptyAndBasePath() {
  console.log('S5: 0 字节与 base-path（空文件 → 200 空 body；--index + --base-path /wesh → /wesh/ 给自定义字节、/ → 404）');
  const tmp = mkTmp('wesh-p9-s5-');
  try {
    // ① 0 字节文件合法（D-07 拒绝列表 = 不存在/不可读/非常规/超限，不含空
    // 文件——伺服空白页是用户明示的整页替换语义）
    const emptyPath = join(tmp, 'empty.html');
    writeFileSync(emptyPath, '');
    const inst = await startWesh(['--index', emptyPath, '--', 'bash', '--norc', '--noprofile']);
    try {
      const r = await rawFetch(inst.port, '/');
      check('S5a', '--index 0 字节文件 → GET / 200 空 body（D-07 拒绝列表不含空文件）',
        r.status === 200 && r.body.length === 0,
        `status=${r.status} bytes=${r.body.length}`);
    } finally {
      inst.kill();
    }

    // ② base-path 组合（契约行 6）：{bp}/ 给自定义字节（StripPrefix 剥前缀后
    // 落装饰层——mux 前缀内自然成立）；bp 外 / 照旧 404（根无挂载）
    const probe = writeProbe(tmp, 'probe.html');
    const inst2 = await startWesh(['--index', probe.path, '--base-path', '/wesh', '--', 'bash', '--norc', '--noprofile']);
    try {
      const r1 = await rawFetch(inst2.port, '/wesh/');
      const r2 = await rawFetch(inst2.port, '/');
      check('S5b', '--index + --base-path /wesh：GET /wesh/ → 200 探针字节且 GET / → 404（根无挂载，契约行 6）',
        r1.status === 200 && r1.body.equals(probe.bytes) && r2.status === 404,
        `bp内=${r1.status} bytes相等=${r1.body.equals(probe.bytes)} 根=${r2.status}`);
    } finally {
      inst2.kill();
    }
  } finally {
    rmTmp(tmp);
  }
}

// ---------- S6：配置通道（D-07：TOML index 同名键铺底 + flag > 配置优先级链） ----------
async function s6ConfigChannel() {
  console.log('S6: 配置通道（TOML index 键（无 CLI）生效；配置 index + CLI --index 另文件 → CLI 覆盖（flag > 配置））');
  const tmp = mkTmp('wesh-p9-s6-');
  try {
    const probe1 = writeProbe(tmp, 'probe.html');
    const probe2 = writeProbe(tmp, 'probe2.html');

    // ① TOML index 键（无 CLI）：配置驱动启动（bind/port/command/index 全 TOML
    // 给出——phase07 S1 配置场景先例形态）
    const cfgA = join(tmp, 'a.toml');
    writeFileSync(cfgA, [
      'bind = "127.0.0.1"',
      'port = 0',
      `index = "${probe1.path}"`,
      'command = ["bash", "--norc", "--noprofile"]',
      '',
    ].join('\n'));
    const inst = await startWesh(['--config', cfgA], { defaultListen: false });
    try {
      const r = await rawFetch(inst.port, '/');
      check('S6a', 'TOML index 键（无 CLI）→ GET / 给自定义字节（D-07 配置同名键铺底）',
        r.status === 200 && r.body.equals(probe1.bytes),
        `status=${r.status} bytes相等=${r.body.equals(probe1.bytes)}`);
    } finally {
      inst.kill();
    }

    // ② 配置 index + CLI --index 另文件 → CLI 覆盖生效（flag > 配置优先级链，
    // 07-06 默认值替换机制承载——配置键换算 flag 注册默认值）
    const cfgB = join(tmp, 'b.toml');
    writeFileSync(cfgB, [
      'bind = "127.0.0.1"',
      'port = 0',
      `index = "${probe1.path}"`,
      'command = ["bash", "--norc", "--noprofile"]',
      '',
    ].join('\n'));
    const inst2 = await startWesh(['--config', cfgB, '--index', probe2.path], { defaultListen: false });
    try {
      const r = await rawFetch(inst2.port, '/');
      check('S6b', '配置 index + CLI --index 另文件 → CLI 覆盖生效（flag > 配置，body 为 CLI 文件字节）',
        r.status === 200 && r.body.equals(probe2.bytes) && !r.body.equals(probe1.bytes),
        `status=${r.status} CLI文件字节=${r.body.equals(probe2.bytes)} 配置文件字节缺席=${!r.body.equals(probe1.bytes)}`);
    } finally {
      inst2.kill();
    }
  } finally {
    rmTmp(tmp);
  }
}

// 输出自净断言（WR-02/review #7 形态——红线由注释纪律升级为运行时自证）：遍历全部
// 已发 detail，断言不含 UAT_CREDENTIAL 值、任一 share token 值（含 '/s/' 链接
// 形态串）与探针内容串（sensitiveTokens 同口径数组——probe 探针 + share token
// + 凭据值全覆盖）；命中即 FAIL。命中时不回显冒犯内容（只打布尔/计数——红线自保）。
function assertOutputClean() {
  const leaked = emittedDetails.some((d) =>
    d.includes(UAT_CREDENTIAL) || d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t)));
  check('SEC', "输出自净：全部 detail 零凭据/token/探针值零 '/s/' 链接形态串（红线运行时自证）",
    !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}

const scenarios = [
  ['S1', s1StartupValidationMatrix],
  ['S2', s2PageServing],
  ['S3', s3GzipVary],
  ['S4', s4AuthSurface],
  ['S5', s5EmptyAndBasePath],
  ['S6', s6ConfigChannel],
];
// 调试场景过滤（PHASE09_ONLY=S1,S3——仅调试用；提交形态恒全场景开启）
const ONLY = process.env.PHASE09_ONLY?.split(',').map((s) => s.trim()) ?? null;
let failed = 0;
for (const [id, s] of scenarios) {
  if (ONLY !== null && !ONLY.includes(id)) continue;
  try {
    await s();
  } catch (e) {
    failed++;
    // 场景异常消息纳入 emittedDetails——assertOutputClean 自净断言面延伸到场景
    // 异常通道（phase07.mjs WR-02 同款纪律）
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
