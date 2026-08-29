// G-08-2 回归夹具（零依赖，Node >= 22 原生 WebSocket/fetch）：README「结构化日志」节
// 两则 journald jq 示例在 systemd 默认配置（StandardOutput=journal）下的可用性回归
// （2026-08-28 UAT 实测发现的 gap——stdout 人读启动横幅与 stderr JSON 事件在 journal
// 合流，jq 遇非 JSON 行 parse error 中止；修复 = README 侧示例统一补 grep '^\{' 预滤，
// wesh 源码零改动，D-14/D-15/D-16 锁定）。
//
// 等价性假设：journalctl -o cat 的 unit 输出 ≡ stdout+stderr 按时序拼接（两路 fd 经
// journal 合流，横幅行先于事件行）——本夹具 spawn 真实二进制分离捕获两路 fd，进程
// 收口后以 stdout 全文在前 + stderr 全文在后拼接模拟合流流；断言管道经 /bin/sh -c
// 以 stdin 喂入合流流（journalctl 段被 stdin 等价替换），grep+jq 段与 README 新示例
// 逐字一致。
//
// 断言四组：①负对照（夹具自证不空转——合流流直灌无防护 jq select 管道必退出非 0
// 且 stderr 含 parse error，证明非 JSON 横幅行确在流中，修复面不是空转通过）；
// ②全流纯度（防护段 + jq . → 退出 0 且 stderr 空——grep 之后每行皆合法 JSON 的
// 机械证明）；③示例一（auth_failed 恰 1 行、零 user/username 键，D-23）；
// ④示例二（attach+detach 恰 2 行、client_id 均 1、detach reason=normal，D-20/D-21）。
//
// 红线（phase08.mjs:18-21 纪律逐字沿用）：share token/凭据值只作断言材料，永不进入
// check detail 或任何控制台输出——detail 只打状态码/布尔/形状/退出码/event 名；
// assertOutputClean 运行时自证零命中。
//
// 运行：node web/uat/phase08-journal.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
import { spawn, spawnSync } from 'node:child_process';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
const SUBPROTOCOL = 'wesh.v1';

// UAT 专用凭据（phase08.mjs 同款；值不入任何输出——红线）；basicAuthHeader 仅作请求构造材料。
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
      ? { version, cols, rows, }
      : { version, cols, rows, ticket })));

const results = [];
// 全部已发 detail 收集（assertOutputClean 遍历材料——WR-02 运行时自净断言）
const emittedDetails = [];
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  emittedDetails.push(String(detail));
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
// 实机工具豁免记录形态：不计失败（与浏览器层 UAT 的 skipped 形态一致）
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
// 断言材料）——红线：token 值永不进 check detail/控制台输出/汇总行。错凭据探针串
// 同口径入本数组（值同样永不进任何输出）。
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
// （phase08.mjs 逐字沿用——本脚本无 unix socket 场景，unix 解析分支保持不消费。）
// opts.defaultListen（默认 true）：前置 --bind 127.0.0.1 --port 0（loopback 随机端口，
// 与用户服务零干扰）；false 时 argv 原样。
// stderr 持续捕获（JSON 事件行断言通道——合流流构造源之一）；stdout 持续捕获（横幅
// 断言通道——合流流构造源之二，启动行解析本身是 D-14 既有消费者形态的活证据）。
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
      // scheme 感知启动行解析（照 phase08.mjs 形态）；ro 行齐备后 50ms 落定窗吸纳 rw 行
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
// （phase08.mjs 逐字沿用——WebSocket 装配双形态分支：无 headers 时数组形态第二参，
// 有 headers 时 Node >= 22 原生 { headers, protocols } 对象形态。）
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

// waitProcClose：child 'close' 事件（进程退出且 stdio 流全闭）决议——waitExit 等价
// 形态（plan 字面）：合流流构造前必须保证 stdout/stderr 捕获全文落定，'exit' 先于
// 流 flush 的竞态由 'close' 结构性消除。恒带超时护栏（phase08.mjs waitExit 同款纪律）。
function waitProcClose(child, timeoutMs) {
  return new Promise((resolve) => {
    const to = setTimeout(() => resolve(false), timeoutMs);
    child.once('close', () => { clearTimeout(to); resolve(true); });
  });
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

// 管道段常量——grep+jq 段与 README「结构化日志」节新示例逐字一致（防漂移条款：
// 一致范围 = 管道形态（journalctl 段→stdin 等价替换 | grep 段 | jq 段的段序与段形）
// + grep '^\{' 防护段字符 + 引号形态——分歧时以 README 为准回改本脚本）。
// select 匹配字面量显式豁免：PIPE_EX2 的 ==1 是夹具确定性参数（D-20——新 spawn 实例
// 唯一成功连接 ⇒ client_id 恒 1），README 示例二的 ==7 数字仅为示例 N，两处数字不同
// 不构成漂移、不得互改——若把本脚本对齐成 ==7，断言 J4 将检出 0 行必挂。
const GREP_GUARD = `grep '^\\{'`;
const PIPE_UNGUARDED = `jq -c 'select(.event=="auth_failed")'`; // 负对照：G-08-2 原缺陷形态（README 修复前示例管道）
const PIPE_PURITY = `${GREP_GUARD} | jq .`;
const PIPE_EX1 = `${GREP_GUARD} | jq -c 'select(.event=="auth_failed")'`;
const PIPE_EX2 = `${GREP_GUARD} | jq -c 'select(.client_id==1)'`;

// 管道输出按行拆分（jq -c 单行紧凑输出形态；尾换行产生的空元素滤除）
const stdoutLines = (r) => r.stdout.split('\n').filter((l) => l.length > 0);
// 安全解析：管道输出行经 grep+jq 双滤后结构性合法 JSON，解析失败返回 null 由断言转 FAIL
const safeParse = (line) => { try { return JSON.parse(line); } catch { return null; } };

// ---------- J：journal 合流模拟回归（G-08-2） ----------
async function journalConfluence() {
  console.log('J: journal 合流模拟（负对照自证 / 全流纯度 / 示例一 auth_failed / 示例二 client_id 关联）');
  // 错凭据探针值同口径入 sensitiveTokens 闭包数组（值永不进任何输出——红线）
  const WRONG = 'uat:wrong-pass-journal-j7';
  sensitiveTokens.push(WRONG);
  const inst = await startWesh(['--credential', UAT_CREDENTIAL, '--', 'bash', '--norc', '--noprofile']);
  try {
    // 事件制造①：错凭据 POST /api/attach → 401 → auth_failed 事件（单次语义独立
    // spawn，04-06 先例——本脚本不制造节流事件，无 429 连发）
    const resp401 = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { Authorization: basicHeader(WRONG) },
    });
    await resp401.text();
    // ①与②之间必设过窗等待（phase08.mjs:473 同款纪律，phase03 场景纪律）：401 即
    // fail#1 武装 1s 节流窗——正确凭据请求若紧随发出则确定性 429 → 无 ticket →
    // 无 attach/detach → 断言 J4 检出 0 行空挂
    await sleep(1150);
    // 事件制造②：正确凭据 → ticket → WS 子协议 + Hello → Welcome → 主动 close →
    // attach/detach 事件对（本实例唯一成功连接 ⇒ client_id == 1 确定，D-20）
    const respOk = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { Authorization: basicAuthHeader() },
    });
    const bodyOk = respOk.status === 200 ? await respOk.json() : {};
    const c = await dialHello(inst.port, { ticket: bodyOk.ticket });
    c.ws.close();
    await waitClose(c.ws, 3000);
    // stderr 落流晚于 close 决议点（05-01 异步流纪律）——waitEvent 等 detach 落流
    const detachSeen = await waitEvent(inst, (m) => m.event === 'detach');
    check('J0', '夹具事件制造：错凭据 401 → auth_failed；正确凭据 attach/detach 对落流且 client_id==1（D-20 夹具确定性）',
      resp401.status === 401 && respOk.status === 200 && detachSeen !== null && detachSeen.client_id === 1,
      `401=${resp401.status === 401} 200=${respOk.status === 200} detach落流=${detachSeen !== null} client_id==1=${detachSeen?.client_id === 1}`);
  } finally {
    inst.kill();
  }
  // 进程收口后构造合流流（journal 时序：横幅在启动期、事件在运行期）：stdout 全文
  // 在前 + stderr 全文在后拼接；stdout 尾换行兜底防两路拼接处行融合
  await waitProcClose(inst.child, 5000);
  const stdoutAll = inst.stdoutText();
  const merged = (stdoutAll.endsWith('\n') ? stdoutAll : stdoutAll + '\n') + inst.stderrText();
  // 管道执行：合流流经 stdin 喂入（journalctl 段被 stdin 等价替换），grep+jq 段与
  // README 新示例逐字一致；timeout 为管道挂死护栏（有限 stdin 下结构性不触达）
  const runPipe = (cmd) => spawnSync('/bin/sh', ['-c', cmd], { input: merged, encoding: 'utf8', timeout: 15000 });

  // ① 负对照（夹具自证不空转）：合流流直灌无防护的 jq select 管道 → 退出码非 0 且
  // stderr 含 parse error——非 JSON 横幅行确在流中，G-08-2 原缺陷形态确被夹具复现
  const neg = runPipe(PIPE_UNGUARDED);
  const negParseErr = (neg.stderr ?? '').includes('parse error');
  check('J1', '负对照：合流流直灌无防护 jq select 管道 → 退出码非 0 且 stderr 含 parse error（夹具确复现 G-08-2 缺陷形态）',
    neg.status !== null && neg.status !== 0 && negParseErr,
    `exit=${neg.status} parseError=${negParseErr}`);

  // ② 全流纯度：防护段 + jq . → 退出 0 且 stderr 空——grep 之后每行皆合法 JSON
  // （「零 parse error」的机械证明）；'{' 起始行数 ≥4 防 vacuous 通过（夹具事件
  // session_start/auth_failed/attach/detach 恰 4 条）
  const pure = runPipe(PIPE_PURITY);
  const jsonLineCount = merged.split('\n').filter((l) => l.startsWith('{')).length;
  check('J2', "全流纯度：防护段 grep '^\\{' + jq . → 退出 0 且 stderr 空（grep 之后每行皆合法 JSON）",
    pure.status === 0 && (pure.stderr ?? '') === '' && jsonLineCount >= 4,
    `exit=${pure.status} stderr空=${(pure.stderr ?? '') === ''} {起始行数=${jsonLineCount}`);

  // ③ 示例一：防护段 + select(.event=="auth_failed") → 退出 0、stdout 恰 1 行、
  // event 命中且无 user/username 键（D-23）、stderr 空
  const ex1 = runPipe(PIPE_EX1);
  const ex1Lines = stdoutLines(ex1);
  const ex1Ev = ex1Lines.length === 1 ? safeParse(ex1Lines[0]) : null;
  const ex1NoUser = ex1Ev !== null && !('user' in ex1Ev) && !('username' in ex1Ev);
  check('J3', '示例一（README 逐字管道）：select(.event=="auth_failed") 恰 1 行且零 user/username 键（D-23）、stderr 空',
    ex1.status === 0 && ex1Lines.length === 1 && ex1Ev?.event === 'auth_failed' && ex1NoUser && (ex1.stderr ?? '') === '',
    `exit=${ex1.status} 行数=${ex1Lines.length} event=auth_failed=${ex1Ev?.event === 'auth_failed'} 无用户名键=${ex1NoUser} stderr空=${(ex1.stderr ?? '') === ''}`);

  // ④ 示例二：防护段 + select(.client_id==1) → 退出 0、stdout 恰 2 行（attach+detach
  // 时序序）、client_id 均 1、detach reason=="normal"（D-20/D-21）、stderr 空
  const ex2 = runPipe(PIPE_EX2);
  const ex2Evs = stdoutLines(ex2).map(safeParse);
  const ex2Names = ex2Evs.map((e) => e?.event ?? '?').join(',');
  const ex2AllId1 = ex2Evs.length === 2 && ex2Evs.every((e) => e?.client_id === 1);
  const ex2Detach = ex2Evs.find((e) => e?.event === 'detach');
  check('J4', '示例二（select 字面量豁免：脚本侧夹具确定性 ==1）：恰 2 行 attach+detach、client_id 均 1、detach reason=="normal"（D-20/D-21）',
    ex2.status === 0 && ex2Names === 'attach,detach' && ex2AllId1 && ex2Detach?.reason === 'normal' && (ex2.stderr ?? '') === '',
    `exit=${ex2.status} 行数=${ex2Evs.length} 事件序=${ex2Names} client_id均1=${ex2AllId1} reason=normal=${ex2Detach?.reason === 'normal'} stderr空=${(ex2.stderr ?? '') === ''}`);
}

// 输出自净断言（WR-02/review #7 形态——红线由注释纪律升级为运行时自证）：遍历全部已发
// detail，断言不含 UAT_CREDENTIAL 值、任一 share token 值（含 '/s/' 链接形态串）与
// 错凭据探针值（同口径 sensitiveTokens 数组）；命中即 FAIL。命中时不回显冒犯
// 内容（只打布尔/计数——红线自保）。
function assertOutputClean() {
  const leaked = emittedDetails.some((d) =>
    d.includes(UAT_CREDENTIAL) || d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t)));
  check('SEC', "输出自净：全部 detail 零凭据/token 值零 '/s/' 链接形态串（红线运行时自证）",
    !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}

// jq 缺失兜底（实机工具豁免纪律，与浏览器层 UAT 的 skipped 形态一致）：合流管道断言
// 依赖系统 jq——不可用则打印 skipped + reason 后退出 0
const jqProbe = spawnSync('jq', ['--version'], { encoding: 'utf8' });
if (jqProbe.error !== undefined || jqProbe.status !== 0) {
  skip('J0', 'jq 可用性前置探测', `jq 不可用（${jqProbe.error?.code ?? `exit ${jqProbe.status}`}）——合流管道断言依赖系统 jq`);
  console.log('\n结果: 0/0 协议断言通过，1 项 skipped（豁免：jq 缺失）');
  process.exit(0);
}

let failed = 0;
try {
  await journalConfluence();
} catch (e) {
  failed++;
  // WR-02：异常消息纳入 emittedDetails——assertOutputClean 自净断言面延伸到场景
  // 异常通道（startWesh 启动超时等消息可携敏感值静默破线）
  emittedDetails.push(String(e.message));
  console.log(`  FAIL  场景异常: ${e.message}`);
}
assertOutputClean();
const skippedN = results.filter((r) => r.ok === null).length;
const passedN = results.filter((r) => r.ok === true).length;
const failedN = results.filter((r) => r.ok === false).length;
console.log(`\n结果: ${passedN}/${results.length - skippedN} 协议断言通过${skippedN ? `，${skippedN} 项 skipped（豁免）` : ''}${failed ? `，${failed} 个场景异常` : ''}`);
process.exit(failedN === 0 && failed === 0 ? 0 : 1);
