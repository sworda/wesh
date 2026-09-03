// Phase 11 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch/child_process/fs）。
// 覆盖 PC-02/PC-03/PC-04 协议面——D-06 八场景一次建齐（10-CONTEXT deferred 项
//「per-client 真实协议行为 UAT 随 Phase 11+ 建设」的兑现；Go 测试 11-01/11-03/11-04
// 证明内部不变量，本脚本证明真实二进制在真实协议面上的端到端行为）：
//   S1 双端独立 pid 互不串台（attach 前 pgrep -P <wesh> 空输出 = 启动期零子进程
//      = PC-02 spawn 点后置直接证据；attach 后两端 echo $$ 回读 pid 不等 +
//      A 唯一标记 B 静默窗零命中）；
//   S2 首帧 winsize = Hello 钳制尺寸（dialHello 111x44 → 首帧 Welcome
//      cols==111/rows==44 + stty size 回读 "44 111"，无 80x24 中间态）；
//   S3 运行期删命令 spawn 失败注入（启动期 LookPath 预检通过后、attach 前
//      unlink argv[0] tmp 副本 → B Error{server_error,"failed to start
//      process"} 逐字 + close 1011；A echo 照常 + /healthz 200——Phase 10 SC4
//      启动期暴露 vs 运行期 per-request degrade 哲学分界实证，Pitfall 5b）；
//   S4 断开 → pgid ESRCH 无僵尸（正常关闭 ws.close(1000) 后 2s 护栏内
//      process.kill(-pid,0) 抛 ESRCH；S4b 1006 真实异常形态 skipped+reason——
//      CODEBUDDY.md 分层测试策略 §5 平台豁免）；
//   S5 EXIT 私有化（A exit 42 → 仅 A 末帧 EXIT exit_code==42 + close 1000；
//      B 1.5s 窗零帧扰动 + 窗后 echo 照常；信号死亡形态 exit_code==-1 + 1000）；
//   S6 --max-clients=1 容量再闸（trap 免疫 linger 注入 → 注册表空出 + pcSessions
//      满窗口 → B 命中 WS 面再闸 Error{server_error,"server is at capacity"}
//      逐字 + close 1011；D-02 wire 形态协议层实证，11-03 Go 侧对照）；
//   S7 断开重连 = 全新进程（pid1 → close → ESRCH → 重连 pid2 ≠ pid1——服务端
//      语义；前端 terminal.reset() 归 Phase 12 不在此断言）；
//   S8 trap '' HUP + --stop-timeout=1s → KILL 兜底（断开后 ~300ms 时点进程组
//      仍存活 → 1s 到期后 5s 护栏内 ESRCH；D-01 机制先行端到端证据）。
//
// 红线（phase06.mjs:11-13 纪律逐字沿用）：token/凭据/pid 数值只作断言材料，永不
// 进入 check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/退出码/
// 文案常量（pid 断言一律以布尔「不等/ESRCH 到达」表达；测试输出可能进 CI 日志，
// 敏感值落盘即泄露样本）。
//
// 时序纪律（phase06.mjs:354-356 容差论证先例）：宽限/退避/免疫类场景真实等待，
// 超时上限只做护栏，禁精确时点断言；时钟不 mock、服务端等待不缩短。
//
// 运行：node web/uat/phase11.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
import { spawn, execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
const EXIT = 0x58; // proto.go Exit='X' 对齐位（S→C 终结帧）
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

// 升档拒绝面断言通道（S3/S6 B 端——无 Welcome，Error 帧 + close 到达即决议；
// phase02.mjs T4b 形态）。返回 {frames, close:{code,reason}}。
function dialExpectReject(port, { cols = 80, rows = 24 } = {}, timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    const to = setTimeout(() => reject(new Error(`dialExpectReject 超时：${timeoutMs}ms 未收到 close`)), timeoutMs);
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame({ cols, rows }));
    ws.onerror = () => { clearTimeout(to); reject(new Error('WS 连接失败')); };
    ws.onclose = (ev) => { clearTimeout(to); resolve({ frames, close: { code: ev.code, reason: ev.reason } }); };
  });
}

const waitClose = (ws, timeoutMs) => new Promise((resolve) => {
  const to = setTimeout(() => resolve(null), timeoutMs);
  ws.onclose = (ev) => { clearTimeout(to); resolve({ code: ev.code, reason: ev.reason }); };
});

// 帧收集器 collectUntilClose(ws)：换装 onmessage/onclose 为本收集器（dialHello 的
// frames 停收——握手后增量帧全归本收集器），close 到达时决议 {frames, close}。
// EXIT 帧序断言形态 = frames 末帧 [0]===EXIT 且 close.code===1000（EXIT 必先于
// 1000——06-RESEARCH Pitfall 1 写序安全的协议层断言）。
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

// EXIT 帧解码（06-RESEARCH Code Examples 逐字形态）
const exitOf = (frames) => JSON.parse(dec.decode(frames.find((f) => f[0] === EXIT).subarray(1)));

// readPid：发 `echo <TAG>=$$\r` 回读会话 pid（phase06.mjs:423-433 正则数字锚定
// 纪律——回显含命令原文（无数字不命中），正则只命中结果行）。pid 数值只作断言
// 材料（红线：永不进 detail——以布尔「解析成功/不等」表达）。
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

// waitScanPid：从持续累积的既有帧轮询扫描 <TAG>=<pid>（S6/S8 非交互死循环夹具
// 启动即印 pid、不读 stdin——pid 经初始 OUTPUT 回读，不发 INPUT）。
const waitScanPid = async (frames, tag) => {
  const t1 = Date.now();
  while (Date.now() - t1 < 5000) {
    const m = new RegExp(`${tag}=(\\d+)`).exec(outputText(frames));
    if (m) return Number(m[1]);
    await sleep(50);
  }
  return null;
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
// 锚点——PATTERNS No Analog ② 新形态首落地）。红线：无错返回 = 存活探针，严禁
// 当死亡证据；EPERM 等意外形态上抛（同用户下不可达）——fail-closed。
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

// ---------- S1：双端独立 pid 互不串台（PC-02 直接证据链） ----------
async function s1DualPidIsolation() {
  console.log('S1: 双端独立 pid 互不串台（启动期零子进程 pgrep 实证 + echo $$ 回读 pid 不等 + A 唯一标记 B 静默窗零命中）');
  const inst = await startWesh(['--session-mode=per-client', '--writable', '--', 'sh']);
  try {
    // S1a 启动期零子进程（PC-02 spawn 点后置实证——main.go run() per-client
    // sess=nil 切换，11-01）：attach 前 pgrep -P <weshPid> 无匹配（exit 1 =
    // 无子进程；有输出 = 子进程存在 = spawn 点前置回归）
    let childless = false;
    try {
      const out = execFileSync('pgrep', ['-P', String(inst.child.pid)], { encoding: 'utf8' });
      childless = out.trim() === '';
    } catch (e) {
      childless = e.status === 1; // pgrep 无匹配退出码 1
    }
    check('S1a', '启动期零子进程：attach 前 pgrep -P <wesh> 无输出（spawn 点后置 = PC-02 直接证据）',
      childless, `零子进程=${childless}`);

    const a = await dialHello(inst.port, {});
    const b = await dialHello(inst.port, {});
    const pidA = await readPid(a.frames, a.ws, 'S1PID');
    const pidB = await readPid(b.frames, b.ws, 'S1PID');
    if (pidA !== null) sensitivePids.push(pidA);
    if (pidB !== null) sensitivePids.push(pidB);
    // S1b 判定方向锚定（phase06.mjs S6c 的反转，PATTERNS §9C——per-client 两端
    // pid **不等** = 独立进程强证据；shared「同 pid 接回」语义勿误抄）
    check('S1b', '双端 echo $$ 回读 pid 不等（attach 即 spawn 独立 PTY 进程强证据）',
      pidA !== null && pidB !== null && pidA !== pidB,
      `解析成功=${pidA !== null && pidB !== null} pid不等=${pidA !== pidB}`);

    // S1c 互不串台：B 先 echo 基线标记回读（排空自身提示符/回显在途帧——「零帧」
    // 断言先排空否则假阳性，11-04 drainQuiet 形态的 UAT 同构），随后 A 发唯一
    // 标记并开 1.5s 静默窗：B 窗内零帧到达且零命中（per-client 输出路径 1:1
    // 直投属主 outbox，A 的任何帧结构性不可达 B）
    const drained = await echoMark(b.frames, b.ws, 'UAT_S1B_BASE_q2w8');
    await sleep(300); // B 落定窗（提示符尾帧吸纳）
    const baseB = b.frames.length;
    const baseA = a.frames.length;
    const MARK = 'UAT_S1_ONLYA_z5x9';
    sendInput(a.ws, `echo ${MARK}\r`);
    const t0 = Date.now();
    let seenA = false;
    let leakedB = false;
    while (Date.now() - t0 < 1500) {
      if (!seenA) seenA = outputText(a.frames, baseA).includes(MARK);
      if (outputText(b.frames, baseB).includes(MARK)) leakedB = true;
      await sleep(50);
    }
    const framesB = b.frames.length - baseB;
    check('S1c', 'A 唯一标记 B 1.5s 静默窗零命中零帧（输出互不串台）',
      drained && seenA && !leakedB && framesB === 0,
      `A回读=${seenA} B零命中=${!leakedB} B窗内帧数=${framesB}`);
    a.ws.close(1000); b.ws.close(1000);
    await waitClose(a.ws, 3000); await waitClose(b.ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S2：首帧 winsize = Hello 钳制尺寸（无 80x24 中间态，PC-03 SC1 后半） ----------
async function s2FirstFrameWinsize() {
  console.log('S2: 首帧 winsize（dialHello 111x44 → 首帧 Welcome cols==111/rows==44 + stty size 回读 "44 111"）');
  const inst = await startWesh(['--session-mode=per-client', '--writable', '--', 'sh']);
  try {
    const c = await dialHello(inst.port, { cols: 111, rows: 44 });
    // 首帧即正确尺寸承载（StartWithSize 直通实证——spawn 消费 Hello 钳制尺寸，
    // 无 80x24 中间态窗口，upgradePerClient Welcome 回显本端 Hello 钳制值）
    const f0 = c.frames[0];
    const w = f0 && f0[0] === WELCOME ? JSON.parse(dec.decode(f0.subarray(1))) : null;
    check('S2a', '首帧 Welcome 且 cols==111/rows==44（无 80x24 中间态——首帧即可断言正确尺寸承载）',
      w !== null && w.cols === 111 && w.rows === 44,
      `首帧Welcome=${w !== null} cols=${w?.cols} rows=${w?.rows}`);
    // PTY 实际尺寸与 Hello 钳制值逐字一致（stty size 输出 rows cols 序；
    // 回显含命令原文 'stty size' 无数字不干扰——"44 111" 只命中结果行）
    const base = c.frames.length;
    sendInput(c.ws, 'stty size\r');
    const t0 = Date.now();
    let hit = false;
    while (Date.now() - t0 < 5000 && !hit) {
      hit = outputText(c.frames, base).includes('44 111');
      if (!hit) await sleep(50);
    }
    check('S2b', 'stty size 回读 "44 111"（rows cols 序——PTY 实际尺寸与 Hello 钳制值逐字一致）',
      hit, `回读命中=${hit}`);
    c.ws.close(1000);
    await waitClose(c.ws, 3000);
  } finally {
    inst.kill();
  }
}

// ---------- S3：运行期删命令 spawn 失败注入（Pitfall 5b 哲学分界实证） ----------
async function s3RuntimeDeletedCommand() {
  console.log('S3: 运行期删命令 spawn 失败（启动期预检通过后 unlink argv0 副本 → B Error+1011；A/服务端零影响）');
  // 注入手法（11-CONTEXT specifics :129）：mkdtemp + cp /bin/sh 副本为 argv0 →
  // 启动期 LookPath 预检通过（10-02 SC4 既成事实——只覆盖启动期）→ A attach
  // 正常 spawn → unlink 副本 → B attach 触发运行期 exec 失败路径（ENOENT）——
  // 启动期 fail-fast vs 运行期 per-request degrade 的哲学分界直接实证
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'wesh-p11-s3-'));
  const argv0copy = path.join(tmp, 'sh-p11');
  fs.copyFileSync('/bin/sh', argv0copy);
  fs.chmodSync(argv0copy, 0o755); // 防御：保证可执行位（copyFileSync 模式保留的实现差异免疫）
  const inst = await startWesh(['--session-mode=per-client', '--writable', '--', argv0copy]);
  try {
    // 删除前 A attach——会话正常（Welcome + echo 标记回读）
    const a = await dialHello(inst.port, {});
    const okBefore = await echoMark(a.frames, a.ws, 'UAT_S3_BEFORE_k7d2');
    check('S3a', '删除前 A attach 会话正常（Welcome + echo 标记回读）', okBefore, `回读=${okBefore}`);
    // 运行期删除 argv0（inode 因 A 会话存活仍在，路径解析已消失）
    fs.unlinkSync(argv0copy);
    const r = await dialExpectReject(inst.port, {});
    const errs = r.frames.filter((f) => f[0] === ERROR);
    const ep = errs.length > 0 ? JSON.parse(dec.decode(errs[0].subarray(1))) : null;
    check('S3b', 'B attach → 恰一帧 Error{server_error, "failed to start process" 逐字} + close 1011',
      errs.length === 1 && ep?.code === 'server_error' && ep?.message === 'failed to start process' && r.close.code === 1011,
      `Error帧数=${errs.length} code=${ep?.code} 文案逐字=${ep?.message === 'failed to start process'} close=${r.close.code}`);
    // 他端与服务端零影响（Pitfall 5 清理清单协议层面）：A echo 照常 + /healthz 200
    const okAfter = await echoMark(a.frames, a.ws, 'UAT_S3_AFTER_m9p4');
    const hz = await fetch(`http://127.0.0.1:${inst.port}/healthz`);
    await hz.text(); // 排空响应体（形状断言只取状态码）
    check('S3c', '他端与服务端零影响（A echo 照常 + /healthz 200）',
      okAfter && hz.status === 200, `A回读=${okAfter} healthz=${hz.status}`);
    a.ws.close(1000);
    await waitClose(a.ws, 3000);
  } finally {
    inst.kill();
    fs.rmSync(tmp, { recursive: true, force: true }); // CI 夹具纪律——tmpdir 清场
  }
}

// ---------- S4：断开 → pgid ESRCH 无僵尸（PC-03/ROADMAP SC3 协议层面） ----------
async function s4DisconnectESRCH() {
  console.log('S4: 断开 → pgid ESRCH 无僵尸（正常关闭 ws.close(1000) 后 2s 护栏内 process.kill(-pid,0) 抛 ESRCH）');
  const inst = await startWesh(['--session-mode=per-client', '--writable', '--', 'sh']);
  try {
    const a = await dialHello(inst.port, {});
    const pid = await readPid(a.frames, a.ws, 'S4PID');
    if (pid !== null) sensitivePids.push(pid);
    a.ws.close(1000);
    // close 握手完成 ⇒ 服务端 detach 已发生（teardown SIGHUP 已发——D-01 序列
    // 起点）；随后轮询 ESRCH 覆盖信号死亡 + Wait 收割全程
    await waitClose(a.ws, 3000);
    const gone = pid !== null && await pollESRCH(pid, 2000);
    check('S4a', '断开后 2s 护栏内 pgid ESRCH（setsid pgid==pid；僵尸未收割则组仍存在——ESRCH = 收割完成强证据）',
      gone, `pid解析=${pid !== null} ESRCH到达=${gone}`);
  } finally {
    inst.kill();
  }
}
// S4b：1006 真实异常断开形态按平台豁免登记 skipped（不阻塞退出码）
function s4bAbnormalExempt() {
  skip('S4b', '1006 真实异常断开形态（OS 网卡栈断网时序）→ pgid ESRCH',
    'CODEBUDDY.md 分层测试策略 §5 平台豁免（真实 OS 断网时序不列阻塞项）+ Node 原生 WebSocket 无 TCP 层强杀面；协议层可覆盖形态 = 正常关闭 + 服务端侧 pgid 断言（S4a）+ 11-01 detach/kick 挂点覆盖论证（一切断开形态同走注册表移除点）+ 11-04 竞态注入测');
}

// 输出自净断言（phase06.mjs review #7 形态延伸——红线由注释纪律升级为运行时
// 自证）：遍历全部已发 detail，断言不含任一 share token 值（含 '/s/' 链接形态串）
// 与任一会话 pid 数值；命中即 FAIL（防未来回归静默破线）。命中时不回显冒犯内容
// （只打布尔/计数——红线自保）。
function assertOutputClean() {
  const leaked = emittedDetails.some((d) =>
    d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t))
    || sensitivePids.some((p) => d.includes(String(p))));
  check('SEC', "输出自净：全部 detail 零 token 值零 pid 数值零 '/s/' 链接形态串（红线运行时自证）",
    !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}

const scenarios = [s1DualPidIsolation, s2FirstFrameWinsize, s3RuntimeDeletedCommand, s4DisconnectESRCH, s4bAbnormalExempt];
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
