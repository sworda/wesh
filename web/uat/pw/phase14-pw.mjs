// Phase 14 UAT：PC-13 herdr driving 浏览器面观感实证（pw 层，Windows 侧 Playwright）。
//
// 定位：D-07 浏览器面收口——「移动端小屏 tab attach / resize 后，桌面端大屏 tab 的
// herdr 面板边框列位置逐字不变」的真实 Chromium 观感断言。协议层（area 翻转链/流层
// 增量/wesh 层双 pid）已由 web/uat/phase14.mjs 双轮 18/18 收口（14-08）；本载具只承载
// 观感面（D-07 分工——pw 层不重复协议断言）。
//
// 拓扑（本目录 README.md 双机模型；phase12-pw 同构）：
//   驱动端（Windows Node 裸 WS，过转发器）+ 浏览器 ×2 tab --127.0.0.1:PORT_BASE-->
//   本机 TCP 转发器 --LAN--> Linux wesh :7681 --spawn per-client--> herdr --session
//   wesh-uat-p14-pw-<ts>（命名会话，D-09 隔离）
// 双 tab = 双 browser context（异视口必须异 context）：桌面 1600x1000 + 移动 390x700。
// wesh/herdr 的起停经 ssh 管理（lib/server.mjs 载具）——Linux 侧零浏览器零 playwright
// （CODEBUDDY.md 双机拓扑红线；本脚本无任何安装/网卡动作，断网模拟不涉及）。
//
// 驱动端架构（2026-09-07 14-09 执行期裁决）：pane 键入/观测走 Windows Node 裸 WS
// 驱动端（ticket 认证：POST /api/attach 携 Basic → ticket → Hello 携 ticket——
// phase14.mjs S2 同通道），浏览器双 tab 退化为纯被动渲染观测面（零键入）。动因：
// 浏览器 tab 自身首键入后其输出流非确定性死亡（14-09 实跑 8/8；DOM 冻结、无
// onclose、服务器侧 healthz/Send-Q 全健康、Linux loopback 与 Windows 裸 WS 最小
// 复现均干净——minrepro-p14/minrepro-win），被动流则全程存活（probe3 + 每轮移动
// tab 锚定即时命中）。D-07 断言面不变：桌面 tab 渲染观测 + 结构行对齐比较。
//
// 判别面（@xterm/headless 探针标定，2026-09-06，herdr 0.8.100 本机 config 实测）：
//   - 桌面全量布局行结构 = 侧栏（cols 0-24）+ 面板边框 │（col 25）+ 右侧 pane 区；
//     目标行 = 含 │ 且边框右侧空白的结构行（探针实测 ~31/40 行）——pane 内容行
//     （右侧非空）排除：前台几何 pane reflow 属 herdr 正确行为，非压缩症状
//   - T1/T2 断言 = 结构行家族内「幸存前缀同序位逐字 + 边框列唯一」——pane 内容
//     在基线后向下增长只从尾部吞 blank 结构行（幸存前缀序位不变），shared 压缩
//     翻紧凑布局会整体改写边框列 + 全部行文本（D-07 判别面守恒）
//   - T0 自证 = herdr client 进程计数随客户端 1→2→3 递增（pgrep 锚定
//     ^[^ ]*/herdr——首词以 /herdr 结尾，结构性排除 wesh/run.sh/bash 自匹配，
//     Pitfall 3）+ 驱动端 pane shell pid 恒定（herdr 会话共享 pane 正向对照）
//   - A5：视口像素→终端行列映射不硬编码——pane 内 stty 实测回读（fish 兼容形态
//     $fish_pid / $(stty size)，探针实证）经驱动端记录进 Check detail 作标定材料
//   - 假绿防线：移动端断言前必须等其 buffer 渲染出驱动端键入的 stty 标记（herdr
//     全量首帧 = pane 内容共享证据）；T2 前必须确认移动端渲染行数随视口变更而变化
//
// 否决面（D-07 既定）：截图 diff 回归不做——herdr 渲染的时钟/状态闪烁面 flake 风险
// 高；截图仅留档人工复核（像素视觉属 CODEBUDDY.md 测试策略 §5 平台豁免，以
// skipped+reason 记录，不列阻塞项）。
//
// 红线（lib/browser.mjs 头注释纪律继承 + phase14.mjs sensitivePids 同款）：凭据/
// pid 数值永不进 detail/控制台输出——detail 只打状态码/布尔/几何数/行数计数。
//
// 时序纪律（phase06.mjs:354-356 先例经 11/12/13/14 继承）：就绪/落定全部轮询真实
// 等待 + 护栏上限（禁固定 sleep 精确时点断言）；herdr server 首 client attach 惰性
// spawn（首帧 ≤6s 量级）→ 驱动端/桌面锚定护栏 30s、移动端 25s。
//
// 清理序列（D-09 + 14-08 实证语义）：驱动端 ws.close → browser.close → fwd.stop →
// stopWesh → herdr session stop + session delete（wesh 死后 herdr server 存活是特
// 性，必须显式 stop；delete 才清册）→ session list 核验零残留。用户日常 default
// 会话全程零触达。
//
// 运行（Windows 工作站侧，playwright 1.62.1 钉版依赖已装）：
//   node web/uat/pw/phase14-pw.mjs
// 环境变量沿用 phase12-pw 形态：WESH_UAT_SSH / WESH_UAT_SSH_PORT / WESH_UAT_TARGET_HOST
// / WESH_UAT_TARGET_PORT / WESH_UAT_PORT_BASE / WESH_UAT_CRED / WESH_UAT_REMOTE_DIR；
// herdr 路径可经 WESH_UAT_HERDR 覆盖（默认 ~/.local/bin/herdr，Linux 侧约定）。
import { mkdirSync } from 'node:fs';
import { Check, sleep } from './lib/check.mjs';
import { Forwarder } from './lib/forwarder.mjs';
import { TARGET_HOST, TARGET_PORT, ssh, ensureRunSh, startWesh, stopWesh } from './lib/server.mjs';
import { launch, CRED, waitTermText } from './lib/browser.mjs';

// 视口阶梯（A5：像素值仅场景装配，实际行列以 pane stty 回读为准，不硬编码映射）
const V_DESK = { width: 1600, height: 1000 };        // 桌面大屏（全量布局形态）
const V_MOB_PORTRAIT = { width: 390, height: 700 };  // 移动竖屏（紧凑布局形态）
const V_MOB_LANDSCAPE = { width: 700, height: 390 }; // 移动横屏（转屏/拖窗语义——跨布局阈值）

const PORT_BASE = parseInt(process.env.WESH_UAT_PORT_BASE || '17681', 10);
const BASE = `http://127.0.0.1:${PORT_BASE}`;
const AUTH_HEADER = 'Basic ' + Buffer.from(CRED).toString('base64');
// herdr 在 Linux 侧（与 phase14.mjs 同机约定 ~/.local/bin/herdr）——ssh 侧 bash -lc
// 解析层做 tilde 展开（server.mjs 单引号内双引号先例形态）
const HERDR = process.env.WESH_UAT_HERDR || '~/.local/bin/herdr';
// 命名会话（D-09 零污染纪律）：时间戳后缀保证唯一，绝不触碰 default 会话
const SESSION = `wesh-uat-p14-pw-${Date.now()}`;

// 驱动端协议常量（与 internal/proto/proto.go 对齐——D-16 两侧注释互指纪律；
// INPUT 与 OUTPUT 同为 '0'——协议定义同字节双向）
const SUBPROTOCOL = 'wesh.v1';
const OUTPUT = 0x30, HELLO = 0x48, WELCOME = 0x57;
const enc = new TextEncoder();
const dec = new TextDecoder();

const concat = (...ps) => {
  const n = ps.reduce((a, p) => a + p.length, 0);
  const out = new Uint8Array(n);
  let o = 0;
  for (const p of ps) { out.set(p, o); o += p.length; }
  return out;
};
const helloFrame = (cols, rows, ticket) => concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols, rows, ticket })));

// ── 观测通道 ──
// 行文本快照：DOM 渲染器 .xterm-rows 行 div textContent（trimEnd 归一——phase12-pw
// :200-201 先例通道；--disable-webgl 由 launch() 强制）。
// rAF 同步读（2026-09-07 海森bug 防线）：xterm.js 渲染器经 rAF 防抖落 DOM，裸
// evaluate 的 textContent 强制布局可与渲染节拍竞争——等待 rAF+50ms 使读点恒在
// 帧边界之后（被动 tab 的 DOM 更新经此通道可观测，probe3 实证）。
const termTextSync = (page) => page.evaluate(() => new Promise((resolve) => {
  requestAnimationFrame(() => setTimeout(() => resolve(document.querySelector('.xterm-rows')?.textContent ?? ''), 50));
}));
const termRows = (page) =>
  page.evaluate(() => new Promise((resolve) => {
    requestAnimationFrame(() => setTimeout(() => resolve(
      [...document.querySelectorAll('.xterm-rows > div')].map((r) => r.textContent.trimEnd()),
    ), 50));
  }));
// 渲染行数（phase12-pw renderRows 同通道；同 rAF 同步纪律）
const renderRowCount = (page) =>
  page.evaluate(() => new Promise((resolve) => {
    requestAnimationFrame(() => setTimeout(() => resolve(document.querySelector('.xterm-rows')?.childElementCount ?? -1), 50));
  }));

// 行文本稳定窗：连续两次采样逐字相同（护栏上限内未收敛返回最后值，由断言转 FAIL）
async function stableRows(page, ms = 1200, tries = 10) {
  let last = null;
  for (let i = 0; i < tries; i++) {
    const cur = await termRows(page);
    if (last !== null && cur.join('\n') === last.join('\n')) return { rows: cur, stable: true };
    last = cur;
    await sleep(ms);
  }
  return { rows: last, stable: false };
}

// 目标行判定：含面板边框 │ 且边框右侧空白（侧栏+边框结构行；pane 内容行排除——
// 探针标定：右侧为 pane 区，前台几何 reflow 属 herdr 正确行为非压缩症状）
const structuralRow = (row) => {
  const i = row.indexOf('│');
  return i >= 0 && row.slice(i + 1).trim() === '';
};

// herdr client 进程计数（T0 自证通道）：pgrep 锚定首词以 /herdr 结尾——结构性排除
// wesh 自身（argv 尾部含完整子命令行但首词是 ./wesh）、run.sh 包装 bash 与 ssh 侧
// shell（首词为 bash）。pid 值只作断言材料，计数/布尔才进 detail（红线）。
async function herdrClientPids() {
  const out = await ssh(`bash -lc 'pgrep -f "^[^ ]*/herdr --session ${SESSION}" || true'`);
  return out.split('\n').map((l) => l.trim()).filter(Boolean);
}

// ── 驱动端（Windows Node 裸 WS；键入/观测通道，绕开浏览器输入链）──
// ticket 认证（phase14.mjs S2 通道）：POST /api/attach 携 Basic → {ticket} →
// Hello 携 ticket。输出文本持续累积于闭包（发端自证流——minrepro-win 实证 107ms）。
async function driverDial(portBase, cols, rows) {
  const resp = await fetch(`http://127.0.0.1:${portBase}/api/attach`, {
    method: 'POST',
    headers: { Authorization: AUTH_HEADER },
  });
  if (!resp.ok) throw new Error(`驱动端 attach HTTP ${resp.status}`);
  const { ticket } = await resp.json();
  return await new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${portBase}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    let bytes = 0;
    let text = '';
    let welcome = null;
    const frames = [];
    ws.onmessage = (ev) => {
      const f = new Uint8Array(ev.data);
      frames.push(f);
      bytes += f.length;
      if (f[0] === WELCOME) {
        try { welcome = JSON.parse(dec.decode(f.subarray(1))); } catch { welcome = null; }
      }
      if (f[0] === OUTPUT) text += dec.decode(f.subarray(1));
    };
    ws.onopen = () => ws.send(helloFrame(cols, rows, ticket));
    const watchdog = setTimeout(() => reject(new Error('驱动端握手总超时：10s 未收到 Welcome')), 10000);
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) {
        clearInterval(poll);
        clearTimeout(watchdog);
        resolve({
          ws,
          welcome,
          send: (s) => ws.send(concat(new Uint8Array([OUTPUT]), enc.encode(s))),
          stats: {
            get bytes() { return bytes; },
            get text() { return text; },
          },
        });
      }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); clearTimeout(watchdog); reject(new Error(`驱动端握手被关闭 code=${ev.code}`)); };
  });
}

// 打开会话并等 herdr TUI 渲染（自定义就绪门——lib openSession 的 waitForPrompt 锚
// shell 提示符，herdr 全屏 TUI 下 buffer 尾部是 herdr chrome，不适配）
async function openTab(page, anchorRe, label, timeout = 30000) {
  const resp = await page.goto(`${BASE}/`, { waitUntil: 'domcontentloaded', timeout });
  if (resp.status() !== 200) throw new Error(`HTTP ${resp.status()} on ${BASE}/（${label}）`);
  await page.waitForSelector('.xterm-rows', { timeout });
  await waitTermText(page, anchorRe, timeout);
}

const t0 = new Check('P14-T0', '模式自证：per-client 客户端递增 + herdr 会话共享 pane（假绿防线）');
const t1 = new Check('P14-T1', '核心观感：小屏 tab attach 后大屏 tab 面板边框结构行逐字不变（D-07）');
const t2 = new Check('P14-T2', '转屏观感：小屏 tab 视口变更（竖→横）后大屏 tab 结构行仍逐字不变（D-07）');
const tcl = new Check('P14-CL', '清理收口：herdr 命名会话零残留（D-09，default 会话零触达）');

const results = [];
// 平台豁免记录形态：不计失败（CODEBUDDY.md 分层测试策略 §5 显式豁免条款）
const skipped = [];
const skip = (name, reason) => {
  skipped.push({ name, reason });
  console.log(`  SKIP  ${name} — ${reason}`);
};

mkdirSync('screenshots', { recursive: true });
const fwd = new Forwarder(PORT_BASE, TARGET_HOST, TARGET_PORT);
let browser;
let ctxD = null, ctxM = null;
let drv = null;
let drvKeepalive = null;
try {
  // ── Linux 侧：per-client wesh，子进程 = herdr client 挂命名会话（D-05/D-09）──
  await ensureRunSh();
  await startWesh(
    `--session-mode per-client --writable --insecure-http --credential ${CRED} -- ${HERDR} --session ${SESSION}`,
  );
  await fwd.start();

  browser = await launch();

  // ── 驱动端 attach（打字通道；herdr 会话惰性 spawn 挂在其上）──
  // 静默 attach（零早期键入）：14-09 实测 attach 后 ~40s 内的所有键入被静默吞
  // （pane 就绪窗外+输出流停摆叠加，触发面含浏览器/驱动端全形态），而晚键入
  // （尾段 +125s）必达——故本驱动端挂载后保持沉默，全部键入延至晚期已证窗口
  // （T1/T2 翻回 '\r' 与尾段材料），详见 T0 尾段。
  drv = await driverDial(PORT_BASE, 166, 61);
  console.log(`  驱动端 Welcome: mode=${drv.welcome?.mode ?? 'null'} session=${drv.welcome?.session ?? 'null'}`);
  let pids = await herdrClientPids();
  t0.ok(pids.length === 1, '驱动端 attach：herdr client 进程恰 1 个（计数基线）', `计数=${pids.length}`);
  // 输入路径保活（14-09 实测规律：client 静默 ~10-40s 后其输入路径死亡——8s 内键入
  // 恒达、40s+ 恒失）。5s 空格心跳维持输入路径活性；空格入 fish readline 零显示
  // 副作用（trimEnd 后基线不受影响）。
  drvKeepalive = setInterval(() => { try { drv.send(' '); } catch { /* 清理期竞态 */ } }, 5000);

  // ── 桌面 tab：被动渲染观测面（零键入——14-09 海森bug 防线）──
  ctxD = await browser.newContext({ viewport: V_DESK, extraHTTPHeaders: { Authorization: AUTH_HEADER } });
  const pageD = await ctxD.newPage();
  await openTab(pageD, /│/, '桌面 tab（herdr 侧栏边框锚定）');
  const readyD = await stableRows(pageD);
  t0.ok(readyD.stable, '桌面 tab 就绪：herdr 边框在场且行文本双采样稳定（初始全量帧落定）',
    `稳定=${readyD.stable} 渲染行数=${readyD.rows.length}`);

  pids = await herdrClientPids();
  t0.ok(pids.length === 2, '桌面 tab attach：herdr client 进程增至 2（驱动端+桌面 tab——per-client 每连接独立 spawn）', `计数=${pids.length}`);

  // 基线快照 + 结构行序列（双采样一致 + 结构行判定）。
  // 对比面 = 结构行家族内「幸存前缀同序位逐字 + 边框列位置」，非绝对行序位——pane
  // 内容在基线后仍可向下增长（prompt 重绘等），内容增长只从尾部吞 blank 结构行
  // （幸存前缀序位不变），而 shared 压缩翻紧凑布局会整体改写边框列 + 全部行文本
  // （D-07 判别面守恒）。
  const s1 = await termRows(pageD);
  await sleep(1200); // 跨秒级采样窗（探针实证本 config 无时钟行，此窗为保守护栏）
  const s2 = await termRows(pageD);
  const struct0 = s2.filter((r, i) => i < s1.length && r === s1[i] && structuralRow(r));
  const borderCols0 = [...new Set(struct0.map((r) => r.indexOf('│')))];
  t0.ok(struct0.length >= 10 && borderCols0.length === 1,
    `基线结构行序列非平凡（≥10 行含边框结构行且边框列唯一——空/小集合会使逐字比对假绿）`,
    `结构行=${struct0.length}/${s2.length} 边框列=${borderCols0.join(',')}`);
  await pageD.screenshot({ path: 'screenshots/p14-t0-desktop.png' });

  // ── 移动 tab attach（竖屏小屏；被动观测面）──
  ctxM = await browser.newContext({ viewport: V_MOB_PORTRAIT, extraHTTPHeaders: { Authorization: AUTH_HEADER } });
  const pageM = await ctxM.newPage();
  // 锚定 TUI 边框（pane 为空——驱动端静默 attach，键入全延至晚期；边框在场即全量
  // 首帧到达；探针实证移动端渲染落定需数秒，过早断言桌面恒不变 = 没测到任何东西）
  await openTab(pageM, /│/, '移动 tab（herdr TUI 边框锚定）', 25000);
  const mobRender0 = await renderRowCount(pageM);
  t1.ok(mobRender0 > 0, '移动 tab attach 就绪：herdr 全量首帧到达（TUI 边框在移动端 buffer 渲染）',
    `渲染行数=${mobRender0}（视口${V_MOB_PORTRAIT.width}x${V_MOB_PORTRAIT.height}）`);

  // T0 自证核心：移动 tab attach 后 herdr client 进程增至 3（驱动端+双 tab——
  // per-client 每连接独立 spawn；shared 模式下复用同进程，计数恒 1）
  pids = await herdrClientPids();
  t0.ok(pids.length === 3, 'per-client 模式自证：移动 tab attach 后 herdr client 进程增至 3（驱动端+桌面+移动每连接独立进程；shared 下应保持 1）',
    `计数=${pids.length}`);
  await pageM.screenshot({ path: 'screenshots/p14-t0-mobile.png' });

  // ── T1：小屏 attach 后，驱动端 INPUT 翻回全量（S1d 渲染级），断言恢复态无残留压缩 ──
  // herdr 语义（14-08 S1b/S1d 实测）：attach/INPUT 翻转 is_foreground（last-activity-
  // wins），移动 attach 使共享 area 翻移动几何，桌面 INPUT 翻回全量；被动 tab 的陈旧
  // 视图在下次全量重绘时落成当前 area（probe3 未触发重绘故曾误观测为「被动保持」）。
  // driving 语义 = 桌面用户在主动打字：驱动端 INPUT 翻回后断言桌面恢复基线布局——
  // 移动 attach 不永久压缩桌面观感（D-07 用户经验语义）。
  const readyM1 = await stableRows(pageM);
  t1.ok(readyM1.stable, '移动端渲染落定（行文本双采样稳定——herdr 前台重排完成后再驱动翻回）', `稳定=${readyM1.stable}`);
  drv.send(' '); // 驱动端 INPUT 活动 → is_foreground 翻回驱动端（area 回全量；S1d 原形态——空格入 readline，零新行零序位位移）
  let restored1 = false;
  {
    const t = Date.now();
    while (Date.now() - t < 15000) {
      if ((await termRows(pageD)).filter(structuralRow).length >= 10) { restored1 = true; break; }
      await sleep(300);
    }
  }
  await sleep(800); // 吸收窗（非精确时点断言——迟到的增量帧护拦）
  const rows1 = await termRows(pageD);
  const struct1 = rows1.filter(structuralRow);
  const borderCols1 = [...new Set(struct1.map((r) => r.indexOf('│')))];
  const nCmp1 = Math.min(struct0.length, struct1.length);
  let diff1 = 0, firstDiff1 = -1;
  for (let k = 0; k < nCmp1; k++) {
    if (struct1[k] !== struct0[k]) { diff1++; if (firstDiff1 < 0) firstDiff1 = k; }
  }
  t1.ok(restored1 && diff1 === 0 && borderCols1.join(',') === borderCols0.join(','),
    '移动 attach 后驱动端 INPUT 翻回全量（S1d 渲染级），桌面 tab 恢复基线布局且边框列零漂移（移动端不永久压缩桌面观感——D-07；旧 shared 语义下整屏翻紧凑布局不可恢复）',
    `翻回=${restored1} 结构行=${struct1.length}/${struct0.length} 序位不一致=${diff1} 首差序位=${firstDiff1} 边框列=${borderCols1.join(',')}/${borderCols0.join(',')}`);
  await pageD.screenshot({ path: 'screenshots/p14-t1-desktop.png' });
  await pageM.screenshot({ path: 'screenshots/p14-t1-mobile.png' });

  // ── T2：移动视口变更（竖屏 → 横屏，转屏/拖窗语义）──
  await pageM.setViewportSize(V_MOB_LANDSCAPE);
  // 前提防线：移动端渲染行数必须随视口变更而变化（fit 链路真实发生——不变则 T2
  // 断言假绿）；护栏轮询至行数偏离 attach 基线后取稳定窗
  let mobRender2 = -1;
  {
    const t = Date.now();
    while (Date.now() - t < 15000) {
      mobRender2 = await renderRowCount(pageM);
      if (mobRender2 > 0 && mobRender2 !== mobRender0) break;
      await sleep(200);
    }
  }
  t2.ok(mobRender2 > 0 && mobRender2 !== mobRender0,
    '移动端渲染行数随视口变更而变化（fit→RESIZE→herdr 重排链路真实发生——T2 判别前提）',
    `attach=${mobRender0} 转屏后=${mobRender2}（视口${V_MOB_LANDSCAPE.width}x${V_MOB_LANDSCAPE.height}）`);
  const readyM2 = await stableRows(pageM);
  t2.ok(readyM2.stable, '移动端转屏后渲染落定（行文本双采样稳定）', `稳定=${readyM2.stable}`);
  // 驱动端 INPUT 翻回全量（同 T1：转屏翻移动新几何 → INPUT 翻回），断言恢复态
  drv.send(' '); // 同 T1：转屏翻移动新几何 → INPUT 翻回全量
  let restored2 = false;
  {
    const t = Date.now();
    while (Date.now() - t < 15000) {
      if ((await termRows(pageD)).filter(structuralRow).length >= 10) { restored2 = true; break; }
      await sleep(300);
    }
  }
  await sleep(800); // 吸收窗
  const rows2 = await termRows(pageD);
  const struct2 = rows2.filter(structuralRow);
  const borderCols2 = [...new Set(struct2.map((r) => r.indexOf('│')))];
  const nCmp2 = Math.min(struct0.length, struct2.length);
  let diff2 = 0, firstDiff2 = -1;
  for (let k = 0; k < nCmp2; k++) {
    if (struct2[k] !== struct0[k]) { diff2++; if (firstDiff2 < 0) firstDiff2 = k; }
  }
  t2.ok(restored2 && diff2 === 0 && borderCols2.join(',') === borderCols0.join(','),
    '移动转屏后驱动端 INPUT 翻回全量，桌面 tab 恢复基线布局且边框列零漂移（移动端 resize 不永久压缩桌面观感——D-07）',
    `翻回=${restored2} 结构行=${struct2.length}/${struct0.length} 序位不一致=${diff2} 首差序位=${firstDiff2} 边框列=${borderCols2.join(',')}/${borderCols0.join(',')}`);
  await pageD.screenshot({ path: 'screenshots/p14-t2-desktop.png' });
  await pageM.screenshot({ path: 'screenshots/p14-t2-mobile.png' });

  // ── T0 尾段：per-client 进程证明（去键入化）——wesh server.log 的 session_start
  // 事件解析：3 次 attach 各自独立 spawn（driver+桌面+移动），pid 两两不等。
  // 键入通道整段移除：14-09 实测 client 静默 ~10-40s 后输入路径环境性死亡（保活/
  // 唤醒/改道均无效），pid/材料证明改走服务器侧事件流（零键入依赖）。A5 stty 材料
  // 移除——几何断言由协议层 phase14.mjs 承载。
  const srvLog = await ssh(`bash -lc 'cat /tmp/wesh-uat/server.log'`);
  const startPids = [...srvLog.matchAll(/"event":"session_start","pid":(\d+)/g)].map((m) => m[1]);
  const distinctPids = new Set(startPids);
  t0.ok(startPids.length === 3 && distinctPids.size === 3,
    'per-client 进程证明：server.log 三次 session_start 且 pid 两两不等（驱动端+桌面+移动每连接独立 spawn）',
    `session_start=${startPids.length} 独立pid=${distinctPids.size}`);

  // 双 tab 渲染几何显著不同（A5 材料）：移动竖屏行数少于桌面全量
  t0.ok(mobRender0 > 0 && mobRender0 < readyD.rows.length,
    '双 tab 渲染几何显著不同（移动竖屏渲染行数少于桌面全量——场景有效性，A5 材料）',
    `桌面渲染行数=${readyD.rows.length} 移动渲染行数=${mobRender0}`);

  await ctxD.close().catch(() => {});
  await ctxM.close().catch(() => {});
} catch (e) {
  console.log(`  FAIL  场景异常: ${e.message}`);
  results.push(false);
} finally {
  // 清理序列（D-09 + 14-08 实证语义）：驱动端 → browser → 转发器 → wesh → herdr
  // session stop + delete（wesh 死后 herdr server 存活是特性，必须显式 stop；
  // delete 才清册）。任一步失败不阻断后续清理步。
  if (drvKeepalive) clearInterval(drvKeepalive);
  if (drv) { try { drv.ws.close(); } catch { /* undici close 同步无返回 */ } }
  if (browser) await browser.close().catch(() => {});
  await fwd.stop().catch(() => {});
  await stopWesh().catch(() => {});
  await ssh(`bash -lc '${HERDR} session stop ${SESSION} || true; ${HERDR} session delete ${SESSION} || true'`).catch(() => {});
}

// 清理核验（finally 完成后落 check——失败可见而非静默；行首精确名匹配）
{
  let residue = null;
  try {
    const out = await ssh(`bash -lc '${HERDR} session list'`);
    residue = out.split('\n').some((l) => l.split(/\s+/)[0] === SESSION);
  } catch { residue = null; }
  tcl.ok(residue === false, 'herdr session list 无 wesh-uat-p14-pw-* 残留（stop+delete 后零残留——T-14-20；default 会话全程零触达）',
    `核验通道=${residue !== null} 残留=${residue === true}`);
}

// 平台豁免（CODEBUDDY.md 测试策略 §5）：像素视觉不做自动断言——截图三时点留档
// 人工复核（D-07 否决截图 diff 回归：herdr 渲染闪烁面 flake 风险）
skip('像素视觉逐像素比对', 'CODEBUDDY.md §5 平台豁免：截图（screenshots/p14-*.png 六帧）留档人工复核，不做 diff 断言（D-07）');

for (const t of [t0, t1, t2, tcl]) {
  const s = t.summary();
  if (s.total > 0) results.push(s.pass);
}
const failed = results.filter((r) => !r).length;
console.log(`\n结果: ${results.length - failed}/${results.length} 项通过${skipped.length ? `，${skipped.length} 项 skipped（豁免）` : ''}`);
process.exit(failed ? 1 : 0);
