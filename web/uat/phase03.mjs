// Phase 3 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch）。
// 覆盖 03-UAT.md 中可用无头客户端验证的认证/TLS/Origin 协议断言（ROADMAP
// 准则 1 ticket 全流程 / 准则 2 爆破节流 / 准则 3 Origin+TLS+安全头）；
// 浏览器原生 Basic 弹窗与凭据缓存、testssl.sh 扫描由人工确认（见 03-UAT.md）。
//
// 红线（SEC-01 延伸到测试输出）：凭据值与 ticket 值只作协议构造材料，
// 永不进入 check detail 或任何控制台输出——detail 只打印状态码/布尔/耗时。
//
// 运行：node web/uat/phase03.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
import { spawn, execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import http from 'node:http';

// 自签证书夹具的测试内正当用途：场景 5 的 fetch wss 需要跳过自签证书校验。
// 这是测试脚本局部行为而非部署教程——生产证书指引见 README（mkcert/CA 方向）。
process.env.NODE_TLS_REJECT_UNAUTHORIZED = '0';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';
const execFileAsync = promisify(execFile);

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
const SUBPROTOCOL = 'wesh.v1';

// UAT 专用凭据（值不入任何输出——红线）；basicAuthHeader 仅作请求构造材料。
const UAT_CREDENTIAL = 'uat:uat-pass-x9';
const basicAuthHeader = () => 'Basic ' + Buffer.from(UAT_CREDENTIAL).toString('base64');

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
const helloFrame = (ticket, version = SUBPROTOCOL) =>
  concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify(
    ticket === undefined
      ? { version, cols: 80, rows: 24 }
      : { version, cols: 80, rows: 24, ticket })));

const results = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok, detail });
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};

// 启动 wesh 实例，解析实际端口，返回 { port, scheme, kill }。
// D-03 适配：所有场景显式 --bind 127.0.0.1（loopback 起服是裸跑/凭据场景的合法前提，
// 默认 0.0.0.0 无凭据自 Phase 3 起拒绝启动）。
function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    let stdoutBuf = '';
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${args.join(' ')}; stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      // IN-04 累积缓冲后匹配（phase05.mjs 形态回填）：listening 行跨 chunk
      // 分块时逐 chunk 直接正则永不命中，8s 超时假失败
      stdoutBuf += d.toString();
      // scheme 感知（03-04 启动行 TLS 场景打印 https://）——http-only 正则会 8s 超时
      const m = /listening on (https?):\/\/[^\s]+:(\d+)/.exec(stdoutBuf);
      if (m) {
        clearTimeout(to);
        resolve({ port: Number(m[2]), scheme: m[1], kill: () => child.kill('SIGKILL'), child });
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

// 拒绝路径 helper（场景 6）：进程预期 3s 内自行退出（启动校验拒绝路径不打印
// listening 行而是直接非零退出，startWesh 等端口必然超时，必须走此 spawn-exit 形态）。
function spawnExpectExit(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, args, { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    child.stderr.on('data', (d) => { stderr += d; });
    const to = setTimeout(() => {
      child.kill('SIGKILL');
      reject(new Error(`wesh 未在 3s 内退出（拒绝路径应早退）: ${args.join(' ')}`));
    }, 3000);
    child.on('exit', (code) => { clearTimeout(to); resolve({ code, stderr }); });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// 建立 WS 连接并完成 Hello 握手（可携 ticket）；返回 { ws, frames }，frames 持续累积
function dialHello(port, { ticket, scheme = 'ws' } = {}) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`${scheme}://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame(ticket));
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

// 手构 WS Upgrade 请求（phase02.mjs 场景 5 形态），resolve HTTP 状态码（101 = 升级成功）
function rawUpgrade(port, headers) {
  return new Promise((resolve, reject) => {
    const key = Buffer.from('uat-origin-probe-key').toString('base64');
    const req = http.request({
      host: '127.0.0.1', port, path: '/ws',
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

// ---------- 场景 1：认证完整链路（准则 1 主链路） ----------
// 节流爬梯 pacing（不可省略）：生产二进制无 throttle flag 覆写通道，退避按 base=1s
// 爬梯 1s→2s→4s（fail#n 后 notBefore=+base<<(n-1)，429 不延长窗口）——每个失败断言
// 后必须真实 sleep 过窗，否则下一请求确定性 429 而非预期 401/200。
async function scenarioAuthFlow() {
  console.log('场景 1: 401 challenge → Basic → ticket → Hello → Welcome → auth_failed');
  const inst = await startWesh(['--credential', UAT_CREDENTIAL, '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    // S1a: GET / 无凭据 → 401 + WWW-Authenticate（整站 Basic 挂 /，D-02；fails=1 → notBefore=+1s）
    let resp = await fetch(`http://127.0.0.1:${inst.port}/`);
    const wwwAuth = resp.headers.get('WWW-Authenticate') ?? '';
    check('S1a', 'GET / 无凭据 → 401 + WWW-Authenticate Basic realm',
      resp.status === 401 && wwwAuth.includes('Basic realm="wesh"'),
      `status=${resp.status} challenge=${wwwAuth.includes('Basic realm="wesh"')}`);
    await resp.text();
    await sleep(1150); // fail#1 窗口 1s 真实过窗

    // S1b: POST /api/attach 无凭据 → 401（fails=2 → notBefore=+2s）
    resp = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, { method: 'POST' });
    const noCredBody = await resp.text();
    const noCredWWW = resp.headers.get('WWW-Authenticate');
    check('S1b', 'POST /api/attach 无凭据 → 401', resp.status === 401, `status=${resp.status}`);
    await sleep(2150); // fail#2 窗口 2s 真实过窗

    // S1c: 错凭据 → 401 且与无凭据完全同文（无枚举 oracle；fails=3 → notBefore=+4s）
    resp = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST',
      headers: { Authorization: 'Basic ' + Buffer.from('uat:wrong-pass').toString('base64') },
    });
    const wrongBody = await resp.text();
    check('S1c', 'POST /api/attach 错凭据 → 401 与无凭据同文（无枚举 oracle）',
      resp.status === 401 && wrongBody === noCredBody && resp.headers.get('WWW-Authenticate') === noCredWWW,
      `status=${resp.status} 同文=${wrongBody === noCredBody}`);
    await sleep(4300); // fail#3 窗口 4s 真实过窗

    // S1d: 正确凭据 → 200 + 非空 ticket + Cache-Control: no-store（recordSuccess 清零节流）
    resp = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { Authorization: basicAuthHeader() },
    });
    const body = resp.status === 200 ? await resp.json() : {};
    const ticketOk = typeof body.ticket === 'string' && body.ticket.length > 0;
    check('S1d', '正确凭据 → 200 + 非空 ticket + Cache-Control no-store',
      resp.status === 200 && ticketOk && (resp.headers.get('Cache-Control') ?? '').includes('no-store'),
      `status=${resp.status} ticket非空=${ticketOk} no-store=${(resp.headers.get('Cache-Control') ?? '').includes('no-store')}`);

    // S1e: Hello 携 ticket → Welcome(mode=rw)（--writable 经 ticket 绑定值下发，D-11）
    const { ws, frames } = await dialHello(inst.port, { ticket: body.ticket });
    const welcome = JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));
    check('S1e', 'Hello 携 ticket → Welcome(mode=rw)', welcome.mode === 'rw', `mode=${welcome.mode}`);
    ws.close();
  } finally {
    inst.kill();
  }

  // S1f: 非法 ticket → Error{auth_failed} + close 1008 reason=auth_failed（D-10 统一口径）。
  // 独立 spawn 实例：隔离节流计数（非法 ticket 核销失败计入 D-08 统一 per-IP 计数器，
  // 与场景 1 主链路的爬梯状态隔离）——多客户端下同进程多 WS 建连已是 Phase 5 特性，
  // 此处独立 spawn 仅为节流隔离；新实例节流状态全新无 pacing 需求。
  const inst2 = await startWesh(['--credential', UAT_CREDENTIAL, '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const ws = new WebSocket(`ws://127.0.0.1:${inst2.port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame('A'.repeat(22))); // 同长度（22 字符）非法 ticket
    const close = await waitClose(ws, 5000);
    const err = frames.find((f) => f[0] === ERROR);
    const errObj = err ? JSON.parse(dec.decode(err.subarray(1))) : null;
    check('S1f', '非法 ticket → Error(auth_failed) + close 1008 reason=auth_failed',
      errObj?.code === 'auth_failed' && close?.code === 1008 && close?.reason === 'auth_failed',
      `code=${errObj?.code} close=${close?.code} reason=${close?.reason ?? ''}`);
  } finally {
    inst2.kill();
  }
}

// ---------- 场景 2：爆破节流（准则 2，SEC-03） ----------
async function scenarioThrottle() {
  console.log('场景 2: 快速连发错凭据 → 429 + Retry-After');
  const inst = await startWesh(['--credential', UAT_CREDENTIAL, '--', 'bash', '--norc', '--noprofile']);
  try {
    // 快速连发 8 次错凭据（<1s 内完成）：fail#1 → 401 且 notBefore=+1s，
    // 后续请求撞退避窗 → 429 + Retry-After（生产 base 1s，无需 sleep 确定性成立）
    const statuses = [];
    let retryAfterSeen = false;
    for (let i = 0; i < 8; i++) {
      const resp = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
        method: 'POST',
        headers: { Authorization: 'Basic ' + Buffer.from(`uat:brute-${i}`).toString('base64') },
      });
      statuses.push(resp.status);
      if (resp.status === 429 && resp.headers.get('Retry-After') !== null) retryAfterSeen = true;
      await resp.text();
    }
    check('S2a', '8 次错凭据连发：首 401，后续撞节流窗 429 + Retry-After',
      statuses[0] === 401 && statuses.slice(1).includes(429) && retryAfterSeen,
      `statuses=${statuses.join(',')} retryAfter=${retryAfterSeen}`);
  } finally {
    inst.kill();
  }
}

// ---------- 场景 3：Origin 白名单（SEC-04，D-12/D-13） ----------
async function scenarioOrigin() {
  console.log('场景 3: Origin 白名单（/api/attach 与 /ws 双端点）');
  const inst = await startWesh(['--credential', UAT_CREDENTIAL, '--origin', 'https://portal.example', '--', 'bash', '--norc', '--noprofile']);
  try {
    // S3a: /api/attach 邪恶 Origin → 403（正确凭据也拒——守卫链 Origin 闸在 Basic 之前）
    let resp = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST',
      headers: { Authorization: basicAuthHeader(), Origin: 'https://evil.example' },
    });
    check('S3a', 'POST /api/attach 邪恶 Origin → 403', resp.status === 403, `status=${resp.status}`);
    await resp.text();

    // S3b: 白名单 Origin → 200
    resp = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST',
      headers: { Authorization: basicAuthHeader(), Origin: 'https://portal.example' },
    });
    check('S3b', 'POST /api/attach 白名单 Origin → 200', resp.status === 200, `status=${resp.status}`);
    await resp.text();

    // S3c: /ws 邪恶 Origin → 403（守卫区 ⓪，Accept 前拒绝）
    const evilStatus = await rawUpgrade(inst.port, { Origin: 'https://evil.example', 'Sec-WebSocket-Protocol': SUBPROTOCOL });
    check('S3c', '/ws 邪恶 Origin → 403', evilStatus === 403, `status=${evilStatus}`);

    // S3d: /ws 无 Origin → 非 403（D-13 非浏览器放行证明；400 = 越过 Origin 闸撞上
    // ①位子协议预检——握手在 Accept 前被拒绝，不建 WS 连接不触发任何会话状态变更；
    // 原断言集中的 409 单客户端门已于 Phase 5 拆除，容量上限断言由 phase05.mjs S5 承接）
    const noOriginStatus = await rawUpgrade(inst.port, {});
    check('S3d', '/ws 无 Origin → 非 403 放行（①位子协议预检 400）',
      noOriginStatus !== 403 && noOriginStatus === 400, `status=${noOriginStatus}`);
  } finally {
    inst.kill();
  }
}

// ---------- 场景 4：无认证模式（loopback 裸跑，D-03 放行面） ----------
async function scenarioNoAuth() {
  console.log('场景 4: 无认证模式（/api/attach 404 + 直连 Welcome）');
  const inst = await startWesh(['--', 'bash', '--norc', '--noprofile']);
  try {
    const resp = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, { method: 'POST' });
    check('S4a', 'POST /api/attach → 404（无认证模式探测信号）', resp.status === 404, `status=${resp.status}`);
    await resp.text();
    const { ws, frames } = await dialHello(inst.port); // Hello 无 ticket 字段
    const welcome = JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));
    check('S4b', 'Hello 无 ticket → Welcome(mode=ro)', welcome.mode === 'ro', `mode=${welcome.mode}`);
    ws.close();
  } finally {
    inst.kill();
  }
}

// ---------- 场景 5：TLS wss 全链路 + 安全头（SEC-05） ----------
async function scenarioTLS() {
  console.log('场景 5: TLS wss 全链路 + HSTS/nosniff/CSP 安全头');
  const dir = mkdtempSync(join(tmpdir(), 'wesh-uat-tls-'));
  try {
    // 自签证书夹具（openssl 1.1.1k 环境清单已核实）；测试内生成即时销毁，无 fixture 文件
    await execFileAsync('openssl', ['req', '-x509', '-newkey', 'rsa:2048',
      '-keyout', join(dir, 'key.pem'), '-out', join(dir, 'cert.pem'),
      '-days', '1', '-nodes', '-subj', '/CN=localhost']);
    const inst = await startWesh(['--credential', UAT_CREDENTIAL,
      '--tls-cert', join(dir, 'cert.pem'), '--tls-key', join(dir, 'key.pem'),
      '--', 'bash', '--norc', '--noprofile']);
    try {
      // S5a: fetch https 无凭据 → 401 + HSTS/nosniff/CSP 三头（fails=1 → notBefore=+1s）
      let resp = await fetch(`https://127.0.0.1:${inst.port}/`);
      const hsts = resp.headers.get('Strict-Transport-Security') ?? '';
      check('S5a', 'TLS GET / 无凭据 → 401 + HSTS/nosniff/CSP 安全头',
        resp.status === 401
          && hsts.includes('max-age=63072000')
          && resp.headers.get('X-Content-Type-Options') === 'nosniff'
          && (resp.headers.get('Content-Security-Policy') ?? '').includes("frame-ancestors 'none'"),
        `status=${resp.status} hsts=${hsts.includes('max-age=63072000')} nosniff=${resp.headers.get('X-Content-Type-Options') === 'nosniff'}`);
      await resp.text();
      await sleep(1150); // 生产节流无 flag 覆写——fail#1 窗口 1s 真实过窗，否则下一请求确定性 429

      // S5b: 正确凭据 POST /api/attach → 200 取 ticket（recordSuccess 清零）
      resp = await fetch(`https://127.0.0.1:${inst.port}/api/attach`, {
        method: 'POST', headers: { Authorization: basicAuthHeader() },
      });
      const body = resp.status === 200 ? await resp.json() : {};
      const ticketOk = typeof body.ticket === 'string' && body.ticket.length > 0;
      check('S5b', 'TLS 正确凭据 → 200 取 ticket', resp.status === 200 && ticketOk, `status=${resp.status} ticket非空=${ticketOk}`);

      // S5c: wss Hello 携 ticket → Welcome（默认 ro）
      const { ws, frames } = await dialHello(inst.port, { ticket: body.ticket, scheme: 'wss' });
      const welcome = JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));
      check('S5c', 'wss Hello 携 ticket → Welcome(mode=ro)', welcome.mode === 'ro', `mode=${welcome.mode}`);
      ws.close();
    } finally {
      inst.kill();
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

// ---------- 场景 6：启动校验矩阵冒烟（D-03/D-05 拒绝路径端到端证据） ----------
async function scenarioStartupMatrix() {
  console.log('场景 6: 启动校验矩阵（拒绝路径 spawn-exit-stderr 断言）');
  // S6a: 默认 bind（0.0.0.0）无凭据 → 3s 内非零退出 + D-03 拒绝文案
  const r1 = await spawnExpectExit(['--', 'cat']);
  check('S6a', '默认 bind 无凭据 → 拒绝启动（D-03）',
    r1.code !== 0 && r1.stderr.includes('refusing to listen on non-loopback'),
    `exit=${r1.code} 文案=${r1.stderr.includes('refusing to listen on non-loopback')}`);

  // S6b: 非 loopback + 凭据 + 无 TLS → 3s 内非零退出 + D-05 拒绝文案
  const r2 = await spawnExpectExit(['--bind', '0.0.0.0', '--credential', UAT_CREDENTIAL, '--', 'cat']);
  check('S6b', '凭据+明文+非 loopback → 拒绝启动（D-05）',
    r2.code !== 0 && r2.stderr.includes('refusing to serve credentials over plaintext'),
    `exit=${r2.code} 文案=${r2.stderr.includes('refusing to serve credentials over plaintext')}`);
}

const scenarios = [scenarioAuthFlow, scenarioThrottle, scenarioOrigin, scenarioNoAuth, scenarioTLS, scenarioStartupMatrix];
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
const passed = results.filter((r) => r.ok).length;
console.log(`\n结果: ${passed}/${results.length} 协议断言通过${failed ? `，${failed} 个场景异常` : ''}`);
process.exit(passed === results.length && failed === 0 ? 0 : 1);
