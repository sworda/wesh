// Phase 5 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch/net）。
// 覆盖 MULTI-01/03/05 与 RES-03 协议面：分享链接全链（ro/rw/错 token/D-05 总闸）、
// 双客户端输出一致、满员 503 双点位、1013 慢消费者踢出（raw-socket stall 夹具）。
// 渲染层多端像素一致性（S7）按 headless 硬约束豁免（CODEBUDDY.md 平台原生行为豁免
// 条款），人工核对清单见 .planning/phases/05-multi-client/05-UAT.md（外部浏览器可执行）。
//
// 红线（phase04.mjs:6-9 纪律逐字沿用）：share token 值只作断言材料，永不进入
// check detail 或任何控制台输出——detail 只打印状态码/布尔/形状（测试输出可能进
// CI 日志，token 落盘即泄露样本）。
//
// 单次语义纪律更新（phase04.mjs:11-12 位置的 Phase 5 改写）：多客户端下同进程
// 多 WS 建连即本 phase 特性；独立 spawn 纪律仅保留给需隔离凭据/容量配置的场景。
//
// 运行：node web/uat/phase05.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
import { spawn } from 'node:child_process';
import net from 'node:net';
import http from 'node:http';
import crypto from 'node:crypto';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
const SUBPROTOCOL = 'wesh.v1';

// UAT 专用凭据（phase03.mjs 同款；值不入任何输出——红线）；basicAuthHeader 仅作请求构造材料。
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
const helloFrame = ({ ticket, version = SUBPROTOCOL, cols = 80, rows = 24 } = {}) =>
  concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify(
    ticket === undefined
      ? { version, cols, rows }
      : { version, cols, rows, ticket })));

const results = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok, detail });
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
// 平台豁免记录形态：不计失败（headless 硬约束，CODEBUDDY.md 显式豁免条款）
const skip = (id, name, reason) => {
  results.push({ id, name, ok: null, detail: reason });
  console.log(`  SKIP  ${id} ${name} — ${reason}`);
};

// 启动 wesh 实例，解析实际端口与分享链接两行，返回 { port, scheme, shareRO, shareRW, stderrText, kill }。
// 所有场景显式 --bind 127.0.0.1 + --port 0（loopback 随机端口，与用户服务零干扰）。
// stdout 三行解析（05-06 启动打印形态）：listening on 行 + share read-only: 行（恒打印）
// + share read-write: 行（仅 --writable，D-05 总闸）——链接即断言材料，token 值只存
// 闭包变量，红线：永不进 check detail/控制台输出。ro 行齐备后 50ms 落定窗吸纳 rw 行
// 可能的管道分块边界；stderr 持续捕获（S6 的 logEvent 断言通道——三要素无敏感串）。
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
      // scheme 感知启动行解析（照 phase03.mjs 形态）
      const m = /listening on (https?):\/\/[^\s]+:(\d+)/.exec(stdoutBuf);
      if (m && stdoutBuf.includes('share read-only:')) {
        settling = true;
        clearTimeout(to);
        setTimeout(() => {
          const shareRO = /share read-only:\s+(\S+)/.exec(stdoutBuf)?.[1] ?? null;
          const shareRW = /share read-write:\s+(\S+)/.exec(stdoutBuf)?.[1] ?? null;
          resolve({ port: Number(m[2]), scheme: m[1], shareRO, shareRW, stderrText: () => stderr, kill: () => child.kill('SIGKILL'), child });
        }, 50);
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// 建立 WS 连接并完成 Hello 握手（可携 ticket、可定尺寸）；返回 { ws, frames }，frames 持续累积
function dialHello(port, { ticket, cols = 80, rows = 24 } = {}) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame({ ticket, cols, rows }));
    ws.onerror = () => reject(new Error('WS 连接失败'));
    // Welcome 到达即视为握手完成
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) { clearInterval(poll); resolve({ ws, frames }); }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); reject(new Error(`握手被关闭 code=${ev.code} reason=${ev.reason}`)); };
  });
}

const waitClose = (ws, timeoutMs) => new Promise((resolve) => {
  const to = setTimeout(() => resolve(null), timeoutMs);
  ws.onclose = (ev) => { clearTimeout(to); resolve({ code: ev.code, reason: ev.reason }); };
});

// 手构 WS Upgrade 请求（phase03.mjs rawUpgrade/phase02.mjs 场景 5 形态），resolve HTTP 状态码（101 = 升级成功）
function rawUpgrade(port, headers) {
  return new Promise((resolve, reject) => {
    const key = crypto.randomBytes(16).toString('base64');
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

// S6 stall 夹具：raw net.Socket 直连手工完成 WS 升级（发送 GET /ws 请求：Host/Upgrade/
// Connection/Sec-WebSocket-Key/Sec-WebSocket-Version: 13/Sec-WebSocket-Protocol 头，读至
// \r\n\r\n 收 101），随后发一帧 masked Hello（0x82 FIN+binary + mask 位 + 4 字节 mask
// XOR 'H'+JSON 载荷，<126 字节单帧）完成注册，立即 socket.pause() 停止一切读取——
// TCP 接收缓冲填满 → 服务端 writer 阻塞 → 默认 512KiB outbox 涨满 → 1013 踢出。
// 纪律登记：stall 必须用 raw socket 而非 Node WebSocket 客户端——undici 实现会持续
// drain TCP，无法制造内核级 stall。
function rawStallClient(port) {
  return new Promise((resolve, reject) => {
    const socket = net.connect(port, '127.0.0.1');
    let buf = Buffer.alloc(0);
    const key = crypto.randomBytes(16).toString('base64');
    const onData = (d) => {
      buf = Buffer.concat([buf, d]);
      const idx = buf.indexOf('\r\n\r\n');
      if (idx === -1) return;
      socket.removeListener('data', onData);
      const head = buf.subarray(0, idx).toString('latin1');
      if (!head.includes(' 101')) { socket.destroy(); reject(new Error(`WS 升级非 101: ${head.split('\r\n')[0]}`)); return; }
      const payload = helloFrame(); // 'H' + {"version","cols":80,"rows":24} JSON = 41 字节（<126 单帧短形）
      const mask = crypto.randomBytes(4);
      const masked = Buffer.allocUnsafe(payload.length);
      for (let i = 0; i < payload.length; i++) masked[i] = payload[i] ^ mask[i & 3];
      socket.write(Buffer.concat([Buffer.from([0x82, 0x80 | payload.length]), mask, masked]));
      socket.pause(); // 内核接收缓冲停止排空——stall 起点
      resolve(socket);
    };
    socket.on('data', onData);
    socket.on('error', reject);
    socket.on('connect', () => {
      socket.write(
        `GET /ws HTTP/1.1\r\nHost: 127.0.0.1:${port}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n` +
        `Sec-WebSocket-Key: ${key}\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Protocol: ${SUBPROTOCOL}\r\n\r\n`);
    });
  });
}

// 取 frames 中首帧 WELCOME 的 JSON 载荷
const welcomeOf = (frames) => JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));

const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');

const outputBytes = (frames, fromIdx = 0) =>
  Buffer.concat(frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => Buffer.from(f.subarray(1))));

// 分享链接 URL → token（/s/{token}/ 路径段；值只作断言材料——红线）
const tokenFromUrl = (url) => /\/s\/([^/]+)\//.exec(url)[1];

// ---------- S1：双客户端输出一致（MULTI-01 协议层） ----------
async function s1DualClientConsistency() {
  console.log('S1: 双客户端输出一致（同 port 两 dialHello 收同一 OUTPUT 流）');
  const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const a = await dialHello(inst.port, { cols: 80, rows: 24 });
    const b = await dialHello(inst.port, { cols: 132, rows: 43 });
    const welcomeA = welcomeOf(a.frames);
    const welcomeB = welcomeOf(b.frames);
    // 断言以两端 Welcome 实际 mode 为准：owner 模式默认下 A=rw / B=ro（D-07 递补降级形态）；
    // 输出一致断言不受 mode 影响
    check('S1a', '双客户端同会话建连：A=rw（owner）/ B=ro（D-07 递补旁观）',
      welcomeA.mode === 'rw' && welcomeB.mode === 'ro',
      `A=${welcomeA.mode} B=${welcomeB.mode}`);

    // 基线：两端均已注册（Welcome 入队即注册完成）后到达的帧才参与一致性断言——
    // 注册前输出不回放（无 ring，D-12 drain 语义）
    const baseA = a.frames.length;
    const baseB = b.frames.length;
    const MARK = 'UAT_S1_DONE_7q2x';
    a.ws.send(concat(new Uint8Array([INPUT]), enc.encode(`seq 1 50000; echo ${MARK}\n`)));
    // 等两端各自收齐同一 payload（marker 回显为齐读信号）
    const t0 = Date.now();
    let doneA = false, doneB = false;
    while (Date.now() - t0 < 15000 && !(doneA && doneB)) {
      if (!doneA) doneA = outputText(a.frames, baseA).includes(MARK);
      if (!doneB) doneB = outputText(b.frames, baseB).includes(MARK);
      if (!doneA || !doneB) await sleep(100);
    }
    await sleep(500); // marker 齐读后等尾随 prompt 帧双端等量落定
    const bytesA = outputBytes(a.frames, baseA);
    const bytesB = outputBytes(b.frames, baseB);
    const identical = bytesA.length === bytesB.length && Buffer.compare(bytesA, bytesB) === 0;
    check('S1b', '两端各自累积收齐同一 OUTPUT payload 逐字节一致',
      doneA && doneB && identical,
      `齐读=${doneA && doneB} 字节数相等=${bytesA.length === bytesB.length} 逐字节=${identical} 量=${bytesA.length}`);
    a.ws.close(); b.ws.close();
    await waitClose(a.ws, 3000); await waitClose(b.ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S2/S3：分享链接全链（ro/rw 同实例）+ D-05 总闸负向 ----------
async function s2s3ShareLinkChains() {
  console.log('S2/S3: 分享链接全链（MULTI-05：GET 页面 → POST token → ticket → Welcome mode）');
  const inst = await startWesh(['--credential', UAT_CREDENTIAL, '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    // --- S2: ro 链接全链 ---
    const pageRO = await fetch(inst.shareRO); // 不带 Authorization
    const chRO = pageRO.headers.get('WWW-Authenticate');
    check('S2a', 'GET ro 链接（无凭据）→ 200 且无 Basic challenge',
      pageRO.status === 200 && chRO === null,
      `status=${pageRO.status} challenge缺席=${chRO === null}`);
    await pageRO.text();

    const respRO = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tokenFromUrl(inst.shareRO) }),
    });
    const bodyRO = respRO.status === 200 ? await respRO.json() : {};
    const ticketROok = typeof bodyRO.ticket === 'string' && bodyRO.ticket.length > 0;
    check('S2b', 'POST /api/attach body 携 ro token → 200 出 ticket',
      respRO.status === 200 && ticketROok,
      `status=${respRO.status} ticket非空=${ticketROok}`);

    const cRO = await dialHello(inst.port, { ticket: bodyRO.ticket });
    const wRO = welcomeOf(cRO.frames);
    check('S2c', 'ro token 全链 → Welcome(mode=ro)', wRO.mode === 'ro', `mode=${wRO.mode}`);
    cRO.ws.close();
    await waitClose(cRO.ws, 3000);

    // --- S3: rw 链接全链（同实例；owner 空位 → rw——ro 端永不占位，D-06） ---
    const pageRW = await fetch(inst.shareRW);
    const chRW = pageRW.headers.get('WWW-Authenticate');
    check('S3a', 'GET rw 链接（无凭据）→ 200 且无 Basic challenge',
      pageRW.status === 200 && chRW === null,
      `status=${pageRW.status} challenge缺席=${chRW === null}`);
    await pageRW.text();

    const respRW = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tokenFromUrl(inst.shareRW) }),
    });
    const bodyRW = respRW.status === 200 ? await respRW.json() : {};
    const ticketRWok = typeof bodyRW.ticket === 'string' && bodyRW.ticket.length > 0;
    check('S3b', 'POST /api/attach body 携 rw token → 200 出 ticket',
      respRW.status === 200 && ticketRWok,
      `status=${respRW.status} ticket非空=${ticketRWok}`);

    const cRW = await dialHello(inst.port, { ticket: bodyRW.ticket });
    const wRW = welcomeOf(cRW.frames);
    check('S3c', 'rw token 全链（owner 空位）→ Welcome(mode=rw)', wRW.mode === 'rw', `mode=${wRW.mode}`);
    cRW.ws.close();
    await waitClose(cRW.ws, 3000);

    // S2d 负面对照（置于全链断言之后）：GET /（无 token）→ 401 challenge（Basic
    // 矩阵不变）。fail#1 产生 +1s throttle 窗口，checkTicket 同样经该闸——负面对照
    // 若提前会使后续 Hello 携票核销撞窗收 auth_failed（实测命中），故排最后。
    const root = await fetch(`http://127.0.0.1:${inst.port}/`);
    const chRoot = root.headers.get('WWW-Authenticate') ?? '';
    check('S2d', 'GET / 无 token → 401 challenge（Basic 矩阵不变）',
      root.status === 401 && chRoot.includes('Basic realm="wesh"'),
      `status=${root.status} challenge=${chRoot.includes('Basic realm="wesh"')}`);
    await root.text();
  } finally {
    inst.kill();
  }

  // S3d: D-05 总闸负向断言——无 --writable spawn 实例 stdout 无 share read-write 行
  const inst2 = await startWesh(['--', 'bash', '--norc', '--noprofile']);
  try {
    check('S3d', '无 --writable → stdout 仅 ro 行、无 rw 行（D-05 总闸）',
      inst2.shareRO !== null && inst2.shareRW === null,
      `ro行=${inst2.shareRO !== null} rw行缺席=${inst2.shareRW === null}`);
  } finally {
    inst2.kill();
  }
}

// ---------- S4：错 token（Basic 矩阵 + 无 oracle 形状断言） ----------
async function s4WrongToken() {
  console.log('S4: 错 token（/s/ 401 challenge + /api/attach 401 无 oracle）');
  // 独立 spawn：需隔离凭据模式 throttle 计数（错 token 失败计入 D-08 统一 per-IP 计数器）
  const inst = await startWesh(['--credential', UAT_CREDENTIAL, '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    const wrong = crypto.randomBytes(16).toString('base64url'); // 22 字符同形异值错 token
    // S4a: GET /s/{错 token}/ → 凭据模式 401 challenge（委托 / 链；fail#1 → +1s 窗口）
    const page = await fetch(`http://127.0.0.1:${inst.port}/s/${wrong}/`);
    const ch = page.headers.get('WWW-Authenticate') ?? '';
    check('S4a', 'GET /s/{错 token}/ → 401 challenge（Basic 矩阵形态）',
      page.status === 401 && ch.includes('Basic realm="wesh"'),
      `status=${page.status} challenge=${ch.includes('Basic realm="wesh"')}`);
    await page.text();
    await sleep(1150); // throttle pacing（phase03.mjs 场景 1 纪律）：fail#1 窗口 1s 真实过窗

    // S4b: POST body 携错 token → 委托原链 401（fail#2 → +2s 窗口）
    const respWrong = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: wrong }),
    });
    const wrongBody = await respWrong.text();
    const wrongWWW = respWrong.headers.get('WWW-Authenticate');
    await sleep(2150); // fail#2 窗口 2s 真实过窗

    // S4c: POST 无 token → 401（形状基准）
    const respNone = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, { method: 'POST' });
    const noneBody = await respNone.text();
    const noneWWW = respNone.headers.get('WWW-Authenticate');
    check('S4b', 'POST 错 token 与无 token 的 401 同文同码（无 oracle，形状级）',
      respWrong.status === 401 && respNone.status === 401 && wrongBody === noneBody && wrongWWW === noneWWW,
      `status=${respWrong.status}/${respNone.status} 同文=${wrongBody === noneBody} 同头=${wrongWWW === noneWWW}`);
  } finally {
    inst.kill();
  }
}

// ---------- S5：满员 503 双点位（RES-03：/api/attach 早闸 + WS ③位） ----------
async function s5FullCapacity503() {
  console.log('S5: 满员 503（--max-clients 1 双点位）');
  const inst = await startWesh(['--max-clients', '1', '--credential', UAT_CREDENTIAL, '--writable', '--', 'bash', '--norc', '--noprofile']);
  try {
    // 占槽：Basic → ticket → WS 注册（R-06 注册后计数，n=1 占满）
    const resp1 = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { Authorization: basicAuthHeader() },
    });
    const body1 = resp1.status === 200 ? await resp1.json() : {};
    const { ws } = await dialHello(inst.port, { ticket: body1.ticket });
    check('S5a', '首客户端占槽成功（--max-clients 1）',
      resp1.status === 200 && ws.readyState === WebSocket.OPEN,
      `attach=${resp1.status}`);

    // S5b: /api/attach 早闸（OQ2）→ 503
    const resp2 = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { Authorization: basicAuthHeader() },
    });
    check('S5b', 'POST /api/attach（满员）→ 503（OQ2 早闸）', resp2.status === 503, `status=${resp2.status}`);
    await resp2.text();

    // S5c: 第二 WS 直连 → ③位 Accept 前 HTTP 503（phase02.mjs 握手拒绝断言先例形态）
    const status = await rawUpgrade(inst.port, { 'Sec-WebSocket-Protocol': SUBPROTOCOL });
    check('S5c', '第二 WS 握手（满员）→ HTTP 503（③位）', status === 503, `status=${status}`);

    ws.close();
    await waitClose(ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S6：1013 慢消费者踢出（raw-socket stall 夹具，review #8 活跃场景） ----------
async function s6SlowConsumerKick() {
  console.log('S6: 1013 踢出（stall 端被踢 + 他人无卡顿 + resume 终结）');
  // 洪水夹具 seq 1 50000000 ≈ 389MB：踢出触发点（stall 端管道 ~10MiB 最坏吸收 +
  // outbox 512KiB 写满）的数量级余量，防子进程先耗尽致 lifecycle 1000 与异步
  // Close(1013) 竞态（05-07 同形态实测裁决：38.9MB 量级不足）。
  const inst = await startWesh(['--', 'seq', '1', '50000000']); // 无 --writable：全 ro → 满即踢，信用门不介入
  try {
    // 第二正常客户端：持续读流（Welcome 后换字节计数 handler——不囤积帧，洪水数百 MB）
    const normal = await dialHello(inst.port, {});
    let normalBytes = 0;
    normal.ws.onmessage = (ev) => { const b = new Uint8Array(ev.data); if (b[0] === OUTPUT) normalBytes += b.length - 1; };

    // stall 客户端：raw socket 手工握手 + masked Hello 注册后 socket.pause() 制造内核级 stall
    const socket = await rawStallClient(inst.port);

    // 断言①：spawn stderr 轮询 10s 内出现 logEvent 行含 code=1013 且 reason=slow_consumer
    //（logEvent 三要素 remote/code/reason 无敏感串，红线不破）
    let kickSeen = false;
    let bytesAtKick = 0;
    const t0 = Date.now();
    while (Date.now() - t0 < 10000) {
      if (inst.stderrText().split('\n').some((l) => l.includes('code=1013') && l.includes('reason=slow_consumer'))) {
        kickSeen = true;
        bytesAtKick = normalBytes;
        break;
      }
      await sleep(100);
    }
    check('S6a', 'stall 端 outbox 写满 → stderr logEvent code=1013 reason=slow_consumer（10s 内）',
      kickSeen, `命中=${kickSeen} 踢出时已收=${bytesAtKick}`);

    // 断言②：同实例第二正常客户端持续推进（踢出后两窗口累积字节单调增长——他人无卡顿）
    await sleep(700);
    const b1 = normalBytes;
    await sleep(700);
    const b2 = normalBytes;
    check('S6b', '第二客户端持续推进（踢出后累积字节单调增长）',
      kickSeen && bytesAtKick > 0 && b1 > bytesAtKick && b2 > b1,
      `窗口=${bytesAtKick}<${b1}<${b2}`);

    // 断言③：断言①②后 socket.resume()，10s 内触发 end/close（服务端 Close 完成后 TCP 终结）
    const ended = await new Promise((resolve) => {
      const to = setTimeout(() => resolve(false), 10000);
      socket.once('end', () => { clearTimeout(to); resolve(true); });
      socket.once('close', () => { clearTimeout(to); resolve(true); });
      socket.resume();
    });
    check('S6c', 'stall 端 resume 后 10s 内 TCP 终结（end/close）', ended, `终结=${ended}`);

    normal.ws.close();
    socket.destroy();
  } finally {
    inst.kill();
  }
}

// ---------- S7：像素层多端一致性（headless 豁免，人工清单指针） ----------
function s7PixelLayerManual() {
  skip('S7', '多客户端像素层渲染一致性（浏览器多端逐屏一致）',
    'headless 硬约束豁免：本机永不具备浏览器，任何自动化（含 playwright）均不可测（CODEBUDDY.md 平台原生行为豁免条款）；人工核对清单 .planning/phases/05-multi-client/05-UAT.md（外部浏览器可执行）');
}

const scenarios = [s1DualClientConsistency, s2s3ShareLinkChains, s4WrongToken, s5FullCapacity503, s6SlowConsumerKick, s7PixelLayerManual];
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
const skipped = results.filter((r) => r.ok === null).length;
const passedN = results.filter((r) => r.ok === true).length;
const failedN = results.filter((r) => r.ok === false).length;
console.log(`\n结果: ${passedN}/${results.length - skipped} 协议断言通过${skipped ? `，${skipped} 项 skipped（豁免）` : ''}${failed ? `，${failed} 个场景异常` : ''}`);
process.exit(failedN === 0 && failed === 0 ? 0 : 1);
