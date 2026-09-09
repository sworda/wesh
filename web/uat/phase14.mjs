// Phase 14 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch/child_process）。
// 覆盖 PC-13 协议面——herdr driving + ro 汇聚两场景（phase13.mjs 同构第四代；
// Windows Playwright 观感层归 phase14-pw.mjs 双机拓扑承载）：
//   S1 herdr driving（D-05/D-06/D-08①/D-09）：wesh --session-mode=per-client
//      --writable -- herdr --session <命名会话>，桌面端 120x40 + 移动端 40x12
//      双 attach——「移动端 attach 桌面端不被压缩」的三通道互证（v1.1 里程碑
//      存在意义的可证伪协议层证据）：
//      - herdr 行为层：HERDR_SOCKET_PATH 定向 herdr api snapshot 的
//        layouts[0].area 翻转链（初始桌面全量 → 移动 attach 翻紧凑 → 桌面
//        INPUT 活动翻回全量 → 移动 RESIZE 50x20 翻新几何）——is_foreground
//        last-activity-wins 仲裁 + per-client area 渲染（RESEARCH §Pattern 3
//        两轮 live spike + 一轮 wesh×herdr 全链 smoke 标定产物）；
//      - wesh 层：stderr 双 session_start 事件（client_id 各异 + pid 不等）+
//        两端 Welcome session=="per-client" + 桌面端 OUTPUT 流光标列几何特征
//        宽于移动端（关系断言）；
//      - 流层：移动 attach 后桌面端只收小增量帧（增量 << 初始全量帧，<5%
//        关系阈值——spike 实测 55B vs 96625B 量级）；移动 resize 后桌面端仍
//        为桌面几何增量更新（非压缩重渲染）。
//   S2 ro 汇聚（D-08②，Pitfall 8 防假绿）：独立实例 share 链接——rw 桌面端
//      + ro 移动端经 herdr 汇聚同会话（POST /api/attach 携 share token 换
//      ticket → Hello JSON 携 ticket 键——唯一通道，WS query 参数无效）；ro
//      端 INPUT 后 herdr pane read --source visible 内容逐字不变 + 标记串
//      缺席（wesh 服务端丢弃 ro INPUT 的实证）；rw 对照端 echo 标记串后
//      pane 可见——FEATURES 裁决 7 文档叙事防说谎防线。
//
// 红线（phase13.mjs:31-34 纪律逐字沿用）：token/凭据/pid 数值只作断言材料，
// 永不进入 check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/
// 退出码/文案常量；herdr 会话隔离（D-09）：全部断言与清理只经 wesh-uat-p14-*
// 命名会话 + HERDR_SOCKET_PATH 定向，用户日常 default 会话零触达（误停即
// 用户环境事故）；清理 finally 兜底（关 WS → wesh SIGTERM → herdr session
// stop/delete → list 核验——wesh 死后 herdr server 存活是特性，必须显式 stop）。
//
// 时序纪律（phase06.mjs:354-356 先例经 phase11/12/13 继承）：就绪/翻转/收流
// 全部轮询真实等待 + 护栏上限（禁固定 sleep 精确时点断言；护栏到期 false 由
// 断言转 FAIL，非无限等待）；herdr server 首 client attach 惰性 spawn
// （attach→全量首帧 ≤6s 量级，RESEARCH §3.6 实测）；时钟不 mock。
//
// 断言纪律：几何断言全为关系谓词（Pitfall 2——area 属于两客户端几何派生集合
// + 翻转方向；禁硬编码 chrome 常量 94/26/39 等，用户 config.toml 共享且
// herdr 版本漂移，DEFAULT_MOBILE_WIDTH_THRESHOLD=64 为几何分岔钉定点）；
// SC2 六项不重复断言（D-13——双 pid 之外的 EXIT 不串台/resize 隔离/ro 门控/
// --once 255/spawn 失败 1011 已由 phase11/12/13.mjs 覆盖，六项重跑证据归
// run-all 承载）。
//
// 运行：node web/uat/phase14.mjs [wesh 二进制路径] [herdr 二进制路径]
//   （默认 /tmp/wesh-uat/wesh 与 ~/.local/bin/herdr；先构建：
//    go build -o /tmp/wesh-uat/wesh ./cmd/wesh。启动打 herdr --version 进
//    日志——诊断材料非断言，版本漂移先核版本纪律 D-09）
import { spawn, execFileSync } from 'node:child_process';
import os from 'node:os';

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';
const HERDR = process.argv[3] ?? `${os.homedir()}/.local/bin/herdr`;

// 会话命名与 socket 推导（D-09 零污染纪律 + RESEARCH §3.1 spike 实证形态）：
// 命名会话 socket = ~/.config/herdr/sessions/<name>/herdr.sock；S1/S2 双场景
// 各自唯一会话（互不复用，失败现场互不污染）
const SESSION1 = `wesh-uat-p14-${process.pid}`;
const SESSION2 = `wesh-uat-p14-${process.pid}-s2`;
const SOCK1 = `${os.homedir()}/.config/herdr/sessions/${SESSION1}/herdr.sock`;
const SOCK2 = `${os.homedir()}/.config/herdr/sessions/${SESSION2}/herdr.sock`;

// 启动打 herdr 版本进日志（诊断材料非断言——失败先核版本纪律 D-09；
// herdr 缺席即环境阻塞：SC3 无替代载具，D-05 既定取舍）
try {
  console.log(`herdr 版本: ${execFileSync(HERDR, ['--version'], { encoding: 'utf8' }).trim()}`);
} catch (e) {
  console.error(`herdr --version 失败（${HERDR}）: ${e.message}`);
  process.exit(1);
}

// 帧类型（与 internal/proto/proto.go 对齐——D-16 两侧注释互指纪律；INPUT 与
// OUTPUT 同为 '0'——协议定义同字节双向）
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
// Hello 载荷 {version,cols,rows[,ticket]}（phase05.mjs 形态：ticket undefined
// 时 JSON 省略字段——omitempty 对称；S2 的 ticket 唯一通道在 Hello JSON 键）
const helloFrame = ({ ticket, version = SUBPROTOCOL, cols = 80, rows = 24 } = {}) =>
  concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify(
    ticket === undefined ? { version, cols, rows } : { version, cols, rows, ticket })));

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

// startWesh 解析 stdout 时把分享链接 token 留入本闭包数组（只作断言材料）——
// 红线：token 值永不进 check detail/控制台输出/汇总行
const sensitiveTokens = [];
// 会话 pid 数值同红线处理（session_start 事件解析结果只作断言材料——pid 断言
// 一律以布尔「不等」表达，数值永不进 detail）
const sensitivePids = [];
// S2 标记串同类管理（echo MARKER 形态——标记串不进 detail，Pitfall 6）
const sensitiveMarkers = [];

// 分享链接 URL → token（/s/{token}/ 路径段；值只作断言材料——红线）
const tokenFromUrl = (url) => /\/s\/([^/]+)\//.exec(url)[1];

// 启动超时 reject 消息脱敏（phase13.mjs:94-98 先例沿用——异常消息进控制台/
// CI 日志，argv 原样回显会把凭据值明文送进日志；防御形态保留）
const redactArgs = (args) => args.map((a, i) => {
  if (a.startsWith('--credential=')) return '--credential=<redacted>';
  if (i > 0 && args[i - 1] === '--credential') return '<redacted>';
  return a;
}).join(' ');

// 启动 wesh 实例（spawn 真实二进制），解析实际端口与分享链接两行，返回
// { port, scheme, shareRO, shareRW, stderrText, kill, child }。显式 --bind
// 127.0.0.1 + --port 0（loopback 随机端口，与用户服务零干扰——phase02.mjs
// D-03 形态）。stdout 两行解析：listening on 行 + share read-only: 行；ro 行
// 齐备后 50ms 落定窗吸纳 rw 行可能的管道分块边界；stderr 持续捕获
// （session_start 事件断言通道）。子进程追踪收口：kill 恒 SIGKILL（CPU 受限
// CI 泄漏级联减速实证纪律——清理序列内的常规 kill 兜底除外）。
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

// 建立 WS 连接并完成 Hello 握手（可定尺寸、可携 ticket）；返回 { ws, frames }，
// frames 持续累积。ws.binaryType='arraybuffer' 纪律（Pitfall 4——默认 'blob'
// 使 new Uint8Array(ev.data) 抛异常、onmessage 静默断链）。Welcome 到达即视为
// 握手完成；10s watchdog 防挂死（IN-04 先例）。
function dialHello(port, { cols = 80, rows = 24, ticket } = {}) {
  return new Promise((resolve, reject) => {
    const url = `ws://127.0.0.1:${port}/ws`;
    const ws = new WebSocket(url, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(helloFrame({ cols, rows, ticket }));
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

// waitExit：child 'exit' 事件决议 {code, signal}——清理序列的 wesh 退出观测
// 通道。恒带超时护栏：被测二进制挂死时护栏到期 resolve(null)，由调用方转
// SIGKILL 兜底，而非无限等待。
function waitExit(child, timeoutMs) {
  return new Promise((resolve) => {
    const to = setTimeout(() => resolve(null), timeoutMs);
    child.once('exit', (code, signal) => { clearTimeout(to); resolve({ code, signal }); });
  });
}

// sendInput / sendResize：INPUT / RESIZE 类型字节 + 载荷组帧发送（RESIZE JSON
// {"cols":C,"rows":R}——proto.DecodeResize 钳制 [1,1000] 在解码层既有，D-16）
const sendInput = (ws, text) => ws.send(concat(new Uint8Array([INPUT]), enc.encode(text)));
const sendResize = (ws, cols, rows) =>
  ws.send(concat(new Uint8Array([RESIZE]), enc.encode(JSON.stringify({ cols, rows }))));

const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');

// outputBytes：OUTPUT 帧载荷字节累计（流层互证的计数通道——fromIdx 支撑
// 「attach/resize 时点后增量」窗口测量）
const outputBytes = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).reduce((n, f) => n + f.length - 1, 0);

const welcomeOf = (frames) => {
  const f = frames.find((fr) => fr[0] === WELCOME);
  return f ? JSON.parse(dec.decode(f.subarray(1))) : null;
};

// maxCursorCol：OUTPUT 文本中全部光标定位序列（CUP/HVP \x1b[<r>;<c>H|f 与
// CHA \x1b[<n>G）的列坐标最大值——per-client 独立渲染的流层几何特征：移动端
// 几何（宽 cols 列）只能寻址 ≤cols 的列坐标，桌面端几何必然出现 >cols 的列
// 坐标（spike 实测移动端 CUP max_col=40 / 桌面端增量 CUP max_col=76 形态）。
// 关系断言材料，禁绝对常量（Pitfall 2）。
const maxCursorCol = (text) => {
  let max = 0;
  for (const m of text.matchAll(/\x1b\[(\d*);(\d*)[Hf]/g)) {
    const col = Number(m[2] || 1);
    if (col > max) max = col;
  }
  for (const m of text.matchAll(/\x1b\[(\d+)G/g)) {
    const col = Number(m[1]);
    if (col > max) max = col;
  }
  return max;
};

// parseEvents：stderr 混合流按行解析 JSON 事件（slog JSON 单行——phase13.mjs
// 形态）——滤非 '{' 起始行（启动行/警告行等人文本成员不算事件）；'{' 起始行
// 非法 JSON 即抛错。事件值只作断言材料——detail 只打 event 名/布尔/计数（红线）。
const parseEvents = (text) =>
  text.split('\n').flatMap((line, i) => {
    if (!line.startsWith('{')) return [];
    try {
      return [JSON.parse(line)];
    } catch (e) {
      throw new Error(`事件行非合法 JSON（第 ${i + 1} 行）: ${line.slice(0, 120)}: ${e.message}`);
    }
  });

// ---------- herdr 观测通道（RESEARCH §Pattern 3 spike 标定产物） ----------
// herdr api snapshot 不接受任何参数，目标会话只能经 HERDR_SOCKET_PATH 环境变量
// 指定（D-09 定向纪律——default 会话零触达的唯一防线）；返回 JSON 的
// result.snapshot.layouts[0].area = {width,height,x,y} 恒等于前台
// （is_foreground，last-activity-wins）客户端的 pane 面积。
const herdrSnap = (sock) => JSON.parse(execFileSync(HERDR, ['api', 'snapshot'],
  { env: { ...process.env, HERDR_SOCKET_PATH: sock }, encoding: 'utf8' }));
const areaOf = (snap) => snap?.result?.snapshot?.layouts?.[0]?.area ?? null;
// focused_pane_id 动态发现（不硬编码 w1:p1——首窗 pane id 形态外漂移时经
// snapshot 自身结构定位，蓝本外形态不静默跟改）
const focusedPaneOf = (snap) => snap?.result?.snapshot?.focused_pane_id ?? null;
// pane 可见内容读回（S2 ro 门控观测通道——text 格式默认）
const paneRead = (sock, paneId) => execFileSync(HERDR, ['pane', 'read', paneId, '--source', 'visible'],
  { env: { ...process.env, HERDR_SOCKET_PATH: sock }, encoding: 'utf8' });

// pollUntil：通用轮询（真实等待 + 护栏上限，禁固定 sleep 精确时点断言——
// phase06.mjs:354-356 时序纪律的同构应用；护栏到期 false 由断言转 FAIL）。
// fn 抛错视为未就绪（如 herdr server 惰性启动期 socket 缺席的
// server_not_running 错误），下轮重试。
const pollUntil = async (fn, guardMs, intervalMs = 100) => {
  const t0 = Date.now();
  while (Date.now() - t0 < guardMs) {
    let ok = false;
    try { ok = await fn(); } catch { ok = false; }
    if (ok) return true;
    await sleep(intervalMs);
  }
  return false;
};

// 几何关系谓词（Pitfall 2 红线——禁硬编码 chrome 常量 94/26/39：chrome 尺寸随
// 用户 config.toml（共享，wesh env 白名单无法隔离）与 herdr 版本漂移；断言只
// 锚定两客户端几何的派生关系与翻转方向）：
// - 紧凑布局 = 移动端几何派生：无侧栏（x==0）且 pane 宽 == 移动端 cols
// - 全量布局 = 桌面端几何派生：有侧栏（x>0）且 移动端 cols < pane 宽 ≤ 桌面端 cols
const isCompactFor = (area, cols) => area !== null && area.x === 0 && area.width === cols;
const isFullFor = (area, deskCols, mobileCols) =>
  area !== null && area.x > 0 && area.width > mobileCols && area.width <= deskCols;

// ---------- 清理序列（D-09 + RESEARCH §3.5 实证语义） ----------
// 关 WS → wesh SIGTERM（5s 护栏 + SIGKILL 兜底）→ herdr session stop（wesh
// 死后 herdr server 存活是特性，必须显式 stop）→ session delete（stopped 行
// 清册——session list 零残留）→ 核验。任一步失败不阻断后续清理步（prohibition：
// 资源泄漏即 DoS 面）；session stop 以会话名为准（ambient HERDR_SOCKET_PATH
// 不干扰——探针实证），default 会话结构性零触达。
const closeWs = async (c, ms = 2000) => {
  if (!c?.ws) return;
  try { c.ws.close(1000); await waitClose(c.ws, ms); } catch { /* 已关闭 */ }
};
const stopWesh = async (inst) => {
  try { inst.child.kill('SIGTERM'); } catch { /* 已退出 */ }
  if (await waitExit(inst.child, 5000) === null) {
    try { inst.child.kill('SIGKILL'); } catch { /* 已退出 */ }
  }
};
const herdrSessionCleanup = (session) => {
  try { execFileSync(HERDR, ['session', 'stop', session], { encoding: 'utf8' }); } catch { /* 已停/不存在 */ }
  try { execFileSync(HERDR, ['session', 'delete', session], { encoding: 'utf8' }); } catch { /* 已删/running 拒删（stop 失败时由核验落 FAIL） */ }
};
// session list 首列精确名匹配（核验通道——行缺席 = 零残留）
const sessionListHas = (session) => {
  try {
    return execFileSync(HERDR, ['session', 'list'], { encoding: 'utf8' })
      .split('\n').some((l) => l.split(/\s+/)[0] === session);
  } catch { return false; }
};

// ---------- S1：herdr driving（PC-13 协议面——「移动端 attach 桌面端不被压缩」三通道互证） ----------
async function s1HerdrDriving() {
  console.log('S1: herdr driving（桌面 120x40 + 移动 40x12 → herdr area 翻转链 + 流层增量 + wesh 层三断言互证「移动端 attach 桌面端不被压缩」）');
  const inst = await startWesh(['--writable', '--session-mode=per-client', '--', HERDR, '--session', SESSION1]);
  let desktop = null, mobile = null;
  try {
    // ① 桌面端 attach 120x40 → 就绪门：桌面初始全量帧到达落定（OUTPUT 字节
    //    >0 且 150ms 双采样相等）且 area 为桌面全量几何派生（Rule 1 实测修正：
    //    「layouts 非空」不充分——server 启动期 layouts[].area 先为 {0,0,0,0}
    //    再入默认布局瞬态 {x>0, width≈默认宽-侧栏}（后者同样满足 isFullFor
    //    谓词），桌面 herdr client 的 size 上报之后才翻桌面几何并推全量帧；
    //    帧到达晚于 size 上报，是最强就绪锚点。移动端必须等此门后再 attach
    //    ——否则桌面 size 上报成为 last-activity，compact-40 永不出现）
    desktop = await dialHello(inst.port, { cols: 120, rows: 40 });
    let area = null;
    let d0 = 0;
    const ready = await pollUntil(() => {
      const now = outputBytes(desktop.frames);
      const settled = now > 0 && now === d0;
      d0 = now;
      if (!settled) return false;
      area = areaOf(herdrSnap(SOCK1));
      return isFullFor(area, 120, 40);
    }, 20000, 150);
    check('S1a', '就绪门：桌面初始全量帧到达落定且 area 为桌面全量几何派生（x>0 且 移动cols<width≤桌面cols——首 attach 即前台）',
      ready && isFullFor(area, 120, 40), `就绪=${ready} 全量形态=${isFullFor(area, 120, 40)}`);

    // ② 移动端 attach 40x12 → area 翻转为移动几何派生紧凑布局（is_foreground
    //    last-activity-wins：attach 即活动）
    mobile = await dialHello(inst.port, { cols: 40, rows: 12 });
    const inc1Mark = desktop.frames.length; // 桌面端增量观测起点（移动 attach 时点）
    const flip1 = await pollUntil(() => { area = areaOf(herdrSnap(SOCK1)); return isCompactFor(area, 40); }, 15000);
    check('S1b', '移动端 attach → area 翻转为移动几何派生紧凑布局（width==移动cols 且 x==0——移动端成前台）',
      flip1 && isCompactFor(area, 40), `翻转=${flip1} 紧凑形态=${isCompactFor(area, 40)}`);

    // ③ 流层互证：移动 attach 后桌面端只收小增量帧（增量 << 初始全量帧——
    //    关系阈值断言，禁绝对常量；spike 实测 55B vs 96625B 量级）
    await sleep(500); // 增量落定窗（翻转已确认后的短 settle，非精确时点断言）
    const inc1 = outputBytes(desktop.frames, inc1Mark);
    check('S1c', '流层互证：移动 attach 后桌面端增量 << 初始全量帧（< 全量×5% 关系阈值——无压缩重渲染）',
      d0 > 0 && inc1 < d0 * 0.05, `增量=${inc1}B < 5%全量=${Math.floor(d0 * 0.05)}B（全量=${d0}B）`);

    // ④ 桌面端 INPUT 注入活动 → area 翻回桌面几何派生（仲裁恢复——v1.1 存在
    //    意义的可证伪证据；pane 输出不翻转前台，只有客户端输入翻转——spike S3/S4 判别）
    sendInput(desktop.ws, ' ');
    const flip2 = await pollUntil(() => { area = areaOf(herdrSnap(SOCK1)); return isFullFor(area, 120, 40); }, 15000);
    check('S1d', '桌面端 INPUT 活动 → area 翻回桌面全量几何派生（x>0 且 移动cols<width≤桌面cols——is_foreground 仲裁恢复生效）',
      flip2 && isFullFor(area, 120, 40), `翻回=${flip2} 全量形态=${isFullFor(area, 120, 40)}`);

    // ⑤ wesh 层：两端 Welcome session=="per-client"（模式位恒序列化——D-08）
    const wDesk = welcomeOf(desktop.frames);
    const wMob = welcomeOf(mobile.frames);
    check('S1e', 'wesh 层：两端 Welcome session 均为 "per-client"（模式位端到端）',
      wDesk?.session === 'per-client' && wMob?.session === 'per-client',
      `桌面=${wDesk?.session} 移动=${wMob?.session}`);

    // ⑥ wesh 层：桌面端 OUTPUT 流光标列几何特征宽于移动端（per-client 独立
    //    渲染——移动端几何只能寻址 ≤40 列，桌面端几何必然出现 >40 列光标定位）
    const deskMaxCol = maxCursorCol(outputText(desktop.frames));
    const mobMaxCol = maxCursorCol(outputText(mobile.frames));
    check('S1f', 'wesh 层：桌面端 OUTPUT 流光标列宽于移动端（deskMax>移动cols 且 mobMax≤移动cols——双端各自几何渲染，桌面端不被压缩）',
      deskMaxCol > 40 && mobMaxCol <= 40 && deskMaxCol > mobMaxCol,
      `桌面maxCol=${deskMaxCol} 移动maxCol=${mobMaxCol}（移动cols=40）`);

    // ⑦ wesh 层：stderr 双 session_start 事件（client_id 各异 + pid 不等——
    //    per-client 双会话进程独立；pid 数值只作断言材料，红线）
    await sleep(500); // 事件落定窗（stderr 管道异步送达）
    const starts = parseEvents(inst.stderrText()).filter((m) => m.event === 'session_start');
    for (const m of starts) sensitivePids.push(m.pid);
    const cidDistinct = starts.length === 2 && starts[0].client_id !== starts[1].client_id;
    const pidDistinct = starts.length === 2 && starts[0].pid !== starts[1].pid;
    check('S1g', 'wesh 层：stderr 恰双 session_start 事件，client_id 各异且 pid 不等（per-client 双独立进程——OPS-12 结构化归因）',
      starts.length === 2 && cidDistinct && pidDistinct,
      `事件数=${starts.length} cid各异=${cidDistinct} pid不等=${pidDistinct}`);

    // ⑧ 移动端 RESIZE 50x20（转屏/拖窗语义）→ area 为新几何派生
    const inc2Mark = desktop.frames.length; // 桌面端增量观测起点（resize 时点）
    sendResize(mobile.ws, 50, 20);
    const flip3 = await pollUntil(() => { area = areaOf(herdrSnap(SOCK1)); return isCompactFor(area, 50); }, 15000);
    check('S1h', '移动端 RESIZE 50x20 → area 为新几何派生紧凑布局（width==新cols 且 x==0——resize 亦前台活动）',
      flip3 && isCompactFor(area, 50), `翻转=${flip3} 新几何=${isCompactFor(area, 50)}`);

    // ⑨ 流层：移动 resize 后桌面端仍为桌面几何增量更新（非压缩重渲染——spike
    //    实测 450B / CUP max_col=76 形态；增量在场 >0 证明桌面视图持续跟踪
    //    pane 内容变化）
    await sleep(500); // 增量落定窗
    const inc2 = outputBytes(desktop.frames, inc2Mark);
    const inc2MaxCol = maxCursorCol(outputText(desktop.frames, inc2Mark));
    check('S1i', '流层：移动 resize 后桌面端仍为桌面几何增量更新（0<增量 < 5%全量 且 光标列>移动新cols——非压缩重渲染）',
      inc2 > 0 && inc2 < d0 * 0.05 && inc2MaxCol > 50,
      `增量=${inc2}B < 5%全量=${Math.floor(d0 * 0.05)}B maxCol=${inc2MaxCol}（移动新cols=50）`);
  } finally {
    // 清理序列（任一步失败不阻断后续清理步）：关 WS ×2 → wesh SIGTERM →
    // herdr session stop/delete（wesh 死后 herdr server 存活是特性，必须显式 stop）
    await closeWs(desktop);
    await closeWs(mobile);
    await stopWesh(inst);
    herdrSessionCleanup(SESSION1);
  }
  // 清理核验（finally 完成后落 check——失败可见而非静默）
  check('S1j', '清理收口：herdr session list 无本会话残留行（stop+delete 后零残留——T-14-17；default 会话全程零触达）',
    !sessionListHas(SESSION1), `残留=${sessionListHas(SESSION1)}`);
}

// ---------- S2：ro 汇聚（D-08②/Pitfall 8 防假绿——FEATURES 裁决 7 文档叙事防说谎防线） ----------
// 独立实例（同 S1 argv 形态，新 SESSION 后缀）：rw 桌面端 + ro 移动端各自持
// 独立 herdr client（分享链接 = 按权限级别的独立进程入场券），经 herdr
// server 汇聚同一会话。ro 门控观测走 herdr pane read（server 侧 pane 可见
// 内容——比流嗅探稳定，D-06 同款理由）；ticket 全链唯一通道 = POST
// /api/attach 携 share token 换 ticket → Hello JSON 携 ticket 键（WS query
// 参数无效且不报错——Pitfall 8，Welcome.mode 自检是防假绿唯一防线）。
async function s2RoConvergence() {
  console.log('S2: ro 汇聚（rw 桌面端 + ro 移动端经 herdr 汇聚同会话——ticket 全链 + pane read 输入门控实证）');
  const RO_MARK = 'UAT_P14_RO_7kq2';
  const RW_MARK = 'UAT_P14_RW_3m9v';
  sensitiveMarkers.push(RO_MARK, RW_MARK); // 标记串同类红线管理（不进 detail）
  const inst = await startWesh(['--writable', '--session-mode=per-client', '--', HERDR, '--session', SESSION2]);
  let rw = null, ro = null;
  try {
    // ① rw 桌面端 ticket 全链（phase05.mjs:286-298 形态）：POST /api/attach 携
    //    rw share token → 一次性 ticket → Hello JSON 携 ticket 键 attach →
    //    Welcome.mode=='rw' 自检落 check
    const respRW = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tokenFromUrl(inst.shareRW) }),
    });
    const bodyRW = respRW.status === 200 ? await respRW.json() : {};
    const ticketRW = typeof bodyRW.ticket === 'string' && bodyRW.ticket.length > 0 ? bodyRW.ticket : null;
    if (ticketRW !== null) sensitiveTokens.push(ticketRW);
    rw = ticketRW !== null ? await dialHello(inst.port, { cols: 120, rows: 40, ticket: ticketRW }) : null;
    const rwMode = rw !== null ? welcomeOf(rw.frames)?.mode : null;
    check('S2a', 'rw ticket 全链：POST /api/attach 携 rw share token → 200 出 ticket → Hello 携 ticket 键 attach → Welcome.mode=="rw"（自检落 check）',
      rw !== null && rwMode === 'rw',
      `status=${respRW.status} ticket非空=${ticketRW !== null} mode=${rwMode ?? '（未 attach）'}`);
    if (rw === null) {
      check('S2b', 'ro ticket 全链 + Welcome.mode=="ro" 自检', false, '前提失败：rw 端未 attach');
      return;
    }

    // ② ro 移动端 ticket 全链 + Welcome.mode=='ro' 自检（Pitfall 8 防假绿核心：
    //    ticket 误塞 WS query 参数不报错但 attach 成 rw——mode 自检是唯一防线）
    const respRO = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tokenFromUrl(inst.shareRO) }),
    });
    const bodyRO = respRO.status === 200 ? await respRO.json() : {};
    const ticketRO = typeof bodyRO.ticket === 'string' && bodyRO.ticket.length > 0 ? bodyRO.ticket : null;
    if (ticketRO !== null) sensitiveTokens.push(ticketRO);
    ro = ticketRO !== null ? await dialHello(inst.port, { cols: 40, rows: 12, ticket: ticketRO }) : null;
    const roMode = ro !== null ? welcomeOf(ro.frames)?.mode : null;
    check('S2b', 'ro ticket 全链：POST /api/attach 携 ro share token → 200 出 ticket → Hello 携 ticket 键 attach → Welcome.mode=="ro"（自检落 check——Pitfall 8：通道误用必现 mode 不符即 FAIL）',
      ro !== null && roMode === 'ro',
      `status=${respRO.status} ticket非空=${ticketRO !== null} mode=${roMode ?? '（未 attach）'}`);
    if (ro === null) return;

    // ③ 就绪门：双端初始帧到达落定（OUTPUT 字节 >0 且 150ms 双采样相等——
    //    双 herdr client 连接完成，foreground 翻转与 pane reflow 均已发生并
    //    吸收进 settle）+ focused_pane_id 动态发现（不硬编码 w1:p1——蓝本外
    //    形态不静默跟改）
    let paneId = null;
    let rw0 = 0, ro0 = 0;
    const ready = await pollUntil(() => {
      const nowRw = outputBytes(rw.frames);
      const rwSettled = nowRw > 0 && nowRw === rw0;
      rw0 = nowRw;
      const nowRo = outputBytes(ro.frames);
      const roSettled = nowRo > 0 && nowRo === ro0;
      ro0 = nowRo;
      if (!rwSettled || !roSettled) return false;
      paneId = focusedPaneOf(herdrSnap(SOCK2));
      return paneId !== null;
    }, 20000, 150);
    check('S2c', '就绪门：双端初始帧到达落定且 focused_pane_id 可发现（rw/ro 双 herdr client 经 server 汇聚同会话）',
      ready && paneId !== null, `就绪=${ready} pane发现=${paneId !== null}`);
    if (paneId === null) return;

    // ④ pane 基线：ro 输入前 pane 可见内容（server 侧结构化观测通道）
    const before = paneRead(SOCK2, paneId);

    // ⑤ ro 端 INPUT（echo RO_MARK 键序列——最简形态，Pitfall 6 fish 兼容）→
    //    护栏窗（若 ro INPUT 未被门控，echo 往返 ~50ms 量级早已反映进 pane；
    //    1500ms 为 30× 余量的缺席断言窗，非精确时点断言）→ pane 逐字不变 +
    //    标记缺席（wesh 服务端丢弃 ro INPUT 的实证——FEATURES 裁决 7 防说谎）
    sendInput(ro.ws, `echo ${RO_MARK}\r`);
    await sleep(1500);
    const afterRO = paneRead(SOCK2, paneId);
    check('S2d', 'ro 门控实证：ro 端 INPUT 后 pane 可见内容与输入前逐字一致且 RO 标记串缺席（wesh 服务端丢弃 ro INPUT——D-08②）',
      before === afterRO && !afterRO.includes(RO_MARK),
      `逐字一致=${before === afterRO} 标记缺席=${!afterRO.includes(RO_MARK)}`);

    // ⑥ rw 对照端 INPUT（echo RW_MARK）→ pane read 含标记（对照面——pane
    //    观测通道本身工作的证据；轮询等待 echo 往返）
    sendInput(rw.ws, `echo ${RW_MARK}\r`);
    const rwVisible = await pollUntil(() => {
      try { return paneRead(SOCK2, paneId).includes(RW_MARK); } catch { return false; }
    }, 10000);
    check('S2e', 'rw 对照：rw 端 INPUT 后 pane 可见内容含 RW 标记串（pane 观测通道本身工作——对照面）',
      rwVisible, `标记到达=${rwVisible}`);

    // ⑦ 终态复合断言：rw 标记在场 + ro 标记仍缺席（门控非瞬时——全程零泄漏）
    let finalRead = '';
    try { finalRead = paneRead(SOCK2, paneId); } catch { /* 观测通道失败由 S2e 承载 */ }
    check('S2f', '终态：pane 含 RW 标记串且 RO 标记串全程缺席（门控与对照双面同读成立）',
      finalRead.includes(RW_MARK) && !finalRead.includes(RO_MARK),
      `RW在场=${finalRead.includes(RW_MARK)} RO缺席=${!finalRead.includes(RO_MARK)}`);
  } finally {
    // 清理序列（同 S1）：关 WS ×2 → wesh SIGTERM → herdr session stop/delete
    await closeWs(rw);
    await closeWs(ro);
    await stopWesh(inst);
    herdrSessionCleanup(SESSION2);
  }
  // 清理核验（finally 完成后落 check——失败可见而非静默）
  check('S2g', '清理收口：herdr session list 无本会话残留行（S2 会话零残留——T-14-17）',
    !sessionListHas(SESSION2), `残留=${sessionListHas(SESSION2)}`);
}

// 输出自净断言（phase13.mjs:740-746 同构 + 标记串扩展——红线由注释纪律升级为
// 运行时自证）：遍历全部已发 detail，断言不含任一 share token/ticket 值（含
// '/s/' 链接形态串）、任一会话 pid 数值与任一标记串值；命中即 FAIL（防未来
// 回归静默破线）。命中时不回显冒犯内容（只打布尔/计数——红线自保）。
function assertOutputClean() {
  const leaked = emittedDetails.some((d) =>
    d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t))
    || sensitivePids.some((p) => d !== '' && d.includes(String(p)))
    || sensitiveMarkers.some((m) => d.includes(m)));
  check('SEC', "输出自净：全部 detail 零 token 值零 pid 数值零标记串零 '/s/' 链接形态串（红线运行时自证）",
    !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}

// 场景串行收口（phase13.mjs:748-769 形态——场景间 300ms + 异常纳入
// emittedDetails + skipped 不阻塞退出码；与头注释两场景清单逐一对账）
const scenarios = [s1HerdrDriving, s2RoConvergence];
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
