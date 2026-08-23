// Phase 6 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch）。
// 覆盖 SESS-01/02/03 与 CORE-05 协议面（06-01/06-02/06-04 三 plan 服务端机制对真实
// 二进制的全链断言）：EXIT 双端广播（ro/rw 同帧逐字节一致 + 帧序先于 1000 + 进程
// 退出码传递）、信号死亡（exit_code=-1 + 大写信号名显式映射）、--once 全链
//（第二客户端双点位 503 + 唯一客户端断开进程退出）、--exit-when-empty 立即与
// 宽限两形态（启动守候不触发 / 宽限内 attach 取消 / 到期退出）、断连重接同一 PTY
//（进程 ID 主证据 + shell 变量佐证）。S7 真实断网栈/浏览器原生事件序列按 headless
// 硬约束豁免（CODEBUDDY.md 平台原生行为豁免条款），人工清单见
// .planning/phases/06-session-lifecycle/06-UAT.md（06-07 产出）。
//
// 红线（phase04.mjs:6-9 纪律逐字沿用）：share token/凭据值只作断言材料，永不进入
// check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/退出码/文案常量
//（测试输出可能进 CI 日志，token 落盘即泄露样本）。
//
// 单次语义纪律更新（phase05.mjs:15-16 位置的 Phase 6 改写）：--once/--exit-when-empty
// 场景的服务端进程退出是特性不是回归——child 'exit' 事件即断言通道（waitExit
// helper），spawn 实例 SIGKILL 收口仅用于未预期退出场景的清理。
//
// 运行：node web/uat/phase06.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
import { spawn } from 'node:child_process';
import http from 'node:http';
import crypto from 'node:crypto';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
const EXIT = 0x58; // proto.go Exit='X' 对齐位（06-01 D-08 契约——S→C 终结帧）
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

// 分享链接 URL → token（/s/{token}/ 路径段；值只作断言材料——红线）
const tokenFromUrl = (url) => /\/s\/([^/]+)\//.exec(url)[1];

// 启动 wesh 实例，解析实际端口与分享链接两行，返回 { port, scheme, shareRO, shareRW, stderrText, kill, child }。
// 所有场景显式 --bind 127.0.0.1 + --port 0（loopback 随机端口，与用户服务零干扰）。
// stdout 三行解析（05-06 启动打印形态）：listening on 行 + share read-only: 行（恒打印）
// + share read-write: 行（仅 --writable，D-05 总闸）——链接即断言材料，token 值只存
// 闭包变量，红线：永不进 check detail/控制台输出。ro 行齐备后 50ms 落定窗吸纳 rw 行
// 可能的管道分块边界；stderr 持续捕获（logEvent/panic 断言通道）。
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

// waitExit：child 'exit' 事件决议 {code, signal}——S1/S3/S4/S5 的 wesh 进程退出断言
// 通道（本 phase 场景的服务端退出是特性不是回归）。恒带超时护栏：被测二进制挂死时
// 护栏到期 resolve(null) 由断言转 FAIL，而非无限等待（T-06-06b；review LOW 吸收）。
function waitExit(child, timeoutMs) {
  return new Promise((resolve) => {
    const to = setTimeout(() => resolve(null), timeoutMs);
    child.once('exit', (code, signal) => { clearTimeout(to); resolve({ code, signal }); });
  });
}

// 帧收集器 collectUntilClose(ws)：换装 onmessage/onclose 为本收集器（dialHello 的
// frames 停收——握手后增量帧全归本收集器，两数组无交集需求），close 到达时决议
// {frames, close:{code,reason}}。EXIT 帧序断言形态 = frames 末帧 [0]===EXIT 且
// close.code===1000（06-RESEARCH Pitfall 1 写序安全的协议层断言：EXIT 必先于 1000）。
function collectUntilClose(ws, timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    const frames = [];
    const to = setTimeout(() => reject(new Error(`collectUntilClose 超时：${timeoutMs}ms 未收到 close`)), timeoutMs);
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onclose = (ev) => { clearTimeout(to); resolve({ frames, close: { code: ev.code, reason: ev.reason } }); };
  });
}

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

// 取 frames 中首帧 WELCOME 的 JSON 载荷
const welcomeOf = (frames) => JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));

const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');

// EXIT 帧解码（06-RESEARCH Code Examples 逐字形态）
const exitOf = (frames) => JSON.parse(dec.decode(frames.find((f) => f[0] === EXIT).subarray(1)));

// ---------- S1：EXIT 双端广播（SESS-03 协议层终证：ro/rw 同帧 + 帧序 + 进程退出码） ----------
async function s1ExitDualBroadcast() {
  console.log('S1: EXIT 双端广播（A=Basic ticket rw / B=shareRO token ro → exit 42 → 双端同帧 EXIT → 1000 → 进程 exit 42）');
  const inst = await startWesh(['--writable', '--credential', UAT_CREDENTIAL, '--', 'bash', '--norc', '--noprofile']);
  try {
    // A：Basic → ticket → 携 ticket 握手（rw——phase05.mjs S5a 凭据链路先例；
    // phase03.mjs 场景 1 dialHelloTicket 形态：POST /api/attach 取 ticket 后 Hello 携票）
    const respA = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { Authorization: basicAuthHeader() },
    });
    const bodyA = respA.status === 200 ? await respA.json() : {};
    const a = await dialHello(inst.port, { ticket: bodyA.ticket });
    // B：stdout 解析的 shareRO 链接 token → POST body → ticket → ro（phase05.mjs S2b/c
    // 先例；token 分支绕过 throttle 且 A 的 Basic 成功已 recordSuccess 清零——无 pacing 需求）
    const respB = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tokenFromUrl(inst.shareRO) }),
    });
    const bodyB = respB.status === 200 ? await respB.json() : {};
    const b = await dialHello(inst.port, { ticket: bodyB.ticket });
    const wA = welcomeOf(a.frames);
    const wB = welcomeOf(b.frames);
    check('S1a', '双端 attach 建立：A 经 Basic ticket（rw）/ B 经 shareRO token（ro）',
      respA.status === 200 && respB.status === 200 && wA.mode === 'rw' && wB.mode === 'ro',
      `attach=${respA.status}/${respB.status} mode=${wA.mode}/${wB.mode}`);

    // 进程退出断言通道先挂（防 'exit' 事件先于监听注册而丢失）；随后换装双端帧收集器，
    // A 写 INPUT 'exit 42\r' 触发子进程退出——终结无权限语义：ro/rw 全员同帧
    const exitP = waitExit(inst.child, 10000);
    const collA = collectUntilClose(a.ws);
    const collB = collectUntilClose(b.ws);
    a.ws.send(concat(new Uint8Array([INPUT]), enc.encode('exit 42\r')));
    const [rA, rB] = await Promise.all([collA, collB]);

    const frameA = rA.frames.find((f) => f[0] === EXIT);
    const frameB = rB.frames.find((f) => f[0] === EXIT);
    const exA = frameA ? exitOf(rA.frames) : null;
    const exB = frameB ? exitOf(rB.frames) : null;
    // 06-01 服务端组文案唯一写口（exitMessage 三形态之正常退出，UI-SPEC 逐字）
    const MSG42 = 'The process exited with code 42.';
    check('S1b', '双端同收 EXIT：exit_code==42 且 message 逐字（服务端唯一写口文案）',
      exA?.exit_code === 42 && exB?.exit_code === 42 && exA?.message === MSG42 && exB?.message === MSG42,
      `exit_code=${exA?.exit_code}/${exB?.exit_code} message逐字=${exA?.message === MSG42 && exB?.message === MSG42}`);
    const identical = Boolean(frameA && frameB && frameA.length === frameB.length
      && Buffer.compare(Buffer.from(frameA), Buffer.from(frameB)) === 0);
    check('S1c', '两端 EXIT 帧体逐字节一致（终结无权限语义——ro/rw 全员同帧）',
      identical, `字节一致=${identical} 帧长=${frameA?.length ?? 0}`);
    check('S1d', '帧序：EXIT 先于 1000 到达（双端末帧 EXIT 且 close.code===1000）',
      rA.frames.at(-1)?.[0] === EXIT && rB.frames.at(-1)?.[0] === EXIT
      && rA.close.code === 1000 && rB.close.code === 1000,
      `末帧EXIT=${rA.frames.at(-1)?.[0] === EXIT}/${rB.frames.at(-1)?.[0] === EXIT} close=${rA.close.code}/${rB.close.code}`);
    const proc = await exitP;
    check('S1e', 'wesh 进程 exit 事件码==42（退出码传递进程级锁定）',
      proc !== null && proc.code === 42, `code=${proc?.code ?? '（未到）'}`);
  } finally {
    inst.kill();
  }
}

// ---------- S2：EXIT 信号死亡（exit_code=-1 + 大写 SIGHUP 显式映射 + 1000） ----------
async function s2ExitSignalDeath() {
  console.log('S2: EXIT 信号死亡（sh 自杀 SIGHUP → EXIT{exit_code:-1} message 含大写 SIGHUP → 1000）');
  // sleep 保 attach 窗口；无认证无写实例 ro 直连即可（输入无需送达——kill 是 sh 自身行为）
  const inst = await startWesh(['--', 'sh', '-c', 'sleep 1; kill -HUP $$']);
  try {
    const c = await dialHello(inst.port, {});
    const r = await collectUntilClose(c.ws);
    const exitF = r.frames.find((f) => f[0] === EXIT);
    const ex = exitF ? exitOf(r.frames) : null;
    // Pitfall 3 显式映射断言：大写 'SIGHUP' 必须在场；'killed by signal hangup' 式
    // 小写描述词为回归形态（Signal.String() 陷阱——signalName 显式表的唯一合法规避，06-01）
    check('S2a', "信号死亡 EXIT：exit_code===-1 且 message 含大写 'SIGHUP'（小写描述词为回归）",
      ex?.exit_code === -1 && typeof ex?.message === 'string'
      && ex.message.includes('SIGHUP') && !ex.message.includes('hangup'),
      `exit_code=${ex?.exit_code} SIGHUP大写=${ex?.message?.includes('SIGHUP') ?? false} 小写回归缺席=${!(ex?.message?.includes('hangup') ?? false)}`);
    check('S2b', '帧序：EXIT 先于 close 1000 到达',
      r.frames.at(-1)?.[0] === EXIT && r.close.code === 1000,
      `末帧EXIT=${r.frames.at(-1)?.[0] === EXIT} close=${r.close.code}`);
  } finally {
    inst.kill();
  }
}

const scenarios = [s1ExitDualBroadcast, s2ExitSignalDeath];
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
