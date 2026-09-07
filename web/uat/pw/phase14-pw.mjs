// Phase 14 UAT：PC-13 herdr driving 浏览器面观感实证（pw 层，Windows 侧 Playwright）。
//
// 定位：D-07 浏览器面收口——「移动端小屏 tab attach / resize 后，桌面端大屏 tab 的
// herdr 面板边框列位置逐字不变」的真实 Chromium 观感断言。协议层（area 翻转链/流层
// 增量/wesh 层双 pid）已由 web/uat/phase14.mjs 双轮 18/18 收口（14-08）；本载具只承载
// 观感面（D-07 分工——pw 层不重复协议断言）。
//
// 拓扑（本目录 README.md 双机模型；phase12-pw 同构）：
//   Windows Chromium ×2 tab --127.0.0.1:PORT_BASE--> 本机 TCP 转发器 --LAN--> Linux wesh
//   :7681 --spawn per-client--> herdr --session wesh-uat-p14-pw-<ts>（命名会话，D-09 隔离）
// 双 tab = 双 browser context（异视口必须异 context）：桌面 1600x1000 + 移动 390x700。
// wesh/herdr 的起停经 ssh 管理（lib/server.mjs 载具）——Linux 侧零浏览器零 playwright
// （CODEBUDDY.md 双机拓扑红线；本脚本无任何安装/网卡动作，断网模拟不涉及）。
//
// 判别面（@xterm/headless 探针标定，2026-09-06，herdr 0.8.100 本机 config 实测）：
//   - 桌面全量布局行结构 = 侧栏（cols 0-24）+ 面板边框 │（col 25）+ 右侧 pane 区；
//     目标行 = 含 │ 且边框右侧空白的结构行（探针实测 ~31/40 行）——pane 内容行
//     （右侧非空）排除：前台几何 pane reflow 属 herdr 正确行为，非压缩症状
//   - T1/T2 断言 = 目标行同序位文本逐字一致（D-07：边框列位置不随移动端 attach/
//     resize 漂移；旧 shared 压缩语义下桌面整屏翻 40 列紧凑布局，全部目标行必变）
//   - T0 自证 = herdr client 进程计数 1→2（pgrep 锚定 ^[^ ]*/herdr——首词以 /herdr
//     结尾，结构性排除 wesh/run.sh/bash 自匹配，Pitfall 3）+ 双 tab pane shell pid
//     相同（herdr 会话共享正向对照）；shared 模式下第二 tab 复用同进程，计数恒 1
//   - A5：视口像素→终端行列映射不硬编码——pane 内 stty 实测回读（fish 兼容形态
//     $fish_pid / $(stty size)，探针实证）记录进 Check detail 作标定材料
//   - 假绿防线：移动端断言前必须等其 buffer 渲染出桌面端键入的 P_B 标记（herdr
//     全量首帧 = pane 内容共享证据——探针实证移动端渲染落定需数秒，过早断言桌面
//     恒不变 = 没测到任何东西）；T2 前必须确认移动端渲染行数随视口变更而变化
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
// spawn（首帧 ≤6s 量级）→ 桌面锚定护栏 30s、移动端 25s。
//
// 清理序列（D-09 + 14-08 实证语义）：browser.close → fwd.stop → stopWesh →
// herdr session stop + session delete（wesh 死后 herdr server 存活是特性，必须显式
// stop；delete 才清册）→ session list 核验零残留。用户日常 default 会话全程零触达。
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
import { launch, CRED, runCmd, waitTermText } from './lib/browser.mjs';

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

const results = [];
// 平台豁免记录（CODEBUDDY.md 测试策略 §5：像素视觉截图留档人工复核，不计失败）
const skipped = [];
const skip = (name, reason) => {
  skipped.push({ name, reason });
  console.log(`  SKIP  ${name} — ${reason}`);
};

// ── 观测通道 ──
// 行文本快照：DOM 渲染器 .xterm-rows 行 div textContent（trimEnd 归一——phase12-pw
// :200-201 先例通道；--disable-webgl 由 launch() 强制）
const termRows = (page) =>
  page.evaluate(() => [...document.querySelectorAll('.xterm-rows > div')].map((r) => r.textContent.trimEnd()));
// 渲染行数（phase12-pw renderRows 同通道）
const renderRowCount = (page) =>
  page.evaluate(() => document.querySelector('.xterm-rows')?.childElementCount ?? -1);

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

// pane 内 fish 兼容标记（探针实证形态）：单发键入 + evaluate 300ms 简单轮询。
// nonce 防命中历史回显；pid 数值只作断言材料（红线）。
// 形态纪律（2026-09-07 实测 14-09 执行期）：单发 + 简单轮询在脚本上下文 15/15 全快
//（probe21/16/17 等，命中 ~373ms）；带 8s 内层重试窗/计数簿记的复合轮询形态 7/7
// 触发 pane 输出流停摆（回显后流死、DOM 冻结、无 onclose 无面板——组件级 A/B 未
// 能隔离出单一触发行，海森bug，14-09-SUMMARY 登记待查）。故本函数锁死已证形态：
// 单发 20s 简单轮询，未观测到则整体重试（新 nonce 重键入），护栏 45s 到期返回
// null 由断言转 FAIL。
async function paneEcho(page, expr, re, timeout = 45000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const nonce = 'P' + Math.random().toString(36).slice(2, 6).toUpperCase();
    await runCmd(page, `echo ${nonce}:${expr}`);
    const typedAt = Date.now();
    while (Date.now() - typedAt < 20000 && Date.now() < deadline) {
      const t = await page.evaluate(() => document.querySelector('.xterm-rows')?.textContent ?? '');
      const m = t.match(new RegExp(`${nonce}:${re.source}`));
      if (m) return m;
      await sleep(300);
    }
  }
  return null;
}

// 打开会话并等 herdr TUI 渲染（自定义就绪门——lib openSession 的 waitForPrompt 锚
// shell 提示符，herdr 全屏 TUI 下 buffer 尾部是 herdr chrome，不适配）
async function openTab(page, anchorRe, label, timeout = 30000) {
  const resp = await page.goto(`${BASE}/`, { waitUntil: 'domcontentloaded', timeout });
  if (resp.status() !== 200) throw new Error(`HTTP ${resp.status()} on ${BASE}/（${label}）`);
  await page.waitForSelector('.xterm-rows', { timeout });
  await waitTermText(page, anchorRe, timeout);
}

const t0 = new Check('P14-T0', '模式自证：per-client 双 tab 独立 herdr client 进程 + 同一 herdr 会话共享（假绿防线）');
const t1 = new Check('P14-T1', '核心观感：小屏 tab attach 后大屏 tab 面板边框结构行逐字不变（D-07）');
const t2 = new Check('P14-T2', '转屏观感：小屏 tab 视口变更（竖→横）后大屏 tab 结构行仍逐字不变（D-07）');
const tcl = new Check('P14-CL', '清理收口：herdr 命名会话零残留（D-09，default 会话零触达）');

mkdirSync('screenshots', { recursive: true });
const fwd = new Forwarder(PORT_BASE, TARGET_HOST, TARGET_PORT);
let browser;
let ctxD = null, ctxM = null;
try {
  // ── Linux 侧：per-client wesh，子进程 = herdr client 挂命名会话（D-05/D-09）──
  await ensureRunSh();
  await startWesh(
    `--session-mode per-client --writable --insecure-http --credential ${CRED} -- ${HERDR} --session ${SESSION}`,
  );
  await fwd.start();

  browser = await launch();

  // ── 桌面 tab：就绪 + 自证材料（全部键入在基线快照之前——pane 新输出只落内容行）──
  // 就绪链刻意不含 renderRowCount：{waitTermText × renderRowCount} 评估对与首键入
  // 的组合实测 6/6 触发 pane 输出流停摆（14-09 执行期海森bug，单因子均干净——
  // probe11/12/21 vs probe10/14/15/19/20 矩阵），稳定性证明由 stableRows 独立承载。
  ctxD = await browser.newContext({ viewport: V_DESK, extraHTTPHeaders: { Authorization: AUTH_HEADER } });
  const pageD = await ctxD.newPage();
  await openTab(pageD, /│/, '桌面 tab（herdr 侧栏边框锚定）');
  const readyD = await stableRows(pageD);
  t0.ok(readyD.stable, '桌面 tab 就绪：herdr 边框在场且行文本双采样稳定（初始全量帧落定）',
    `稳定=${readyD.stable} 渲染行数=${readyD.rows.length}`);

  let pids = await herdrClientPids();
  t0.ok(pids.length === 1, '桌面 tab 单独在场：herdr client 进程恰 1 个（计数通道自证）', `计数=${pids.length}`);

  // 首键入前 settle：herdr 惰性 spawn + fish（NVM 启动 ~2-3s）就绪留缓冲，使键入
  // 落在 pane 可交互后、回显与 prompt 重绘全部落在基线快照之前（时序纪律：护栏内
  // 真实等待，非精确时点断言）。
  await sleep(8000);

  // pane 交互通道 + A5 标定材料（fish 兼容形态，探针实证）：pid 与桌面派生 pane 几何
  const pidA = (await paneEcho(pageD, '$fish_pid', '(\\d+)'))?.[1] ?? null;
  const sttyD = await paneEcho(pageD, '$(stty size)', '(\\d+) (\\d+)');
  const deskPane = sttyD ? { rows: +sttyD[1], cols: +sttyD[2] } : null;
  t0.ok(pidA !== null && deskPane !== null, '桌面 pane 交互就绪：fish 标记回读 pid + stty 几何（A5 标定材料）',
    `pid可读=${pidA !== null} pane几何=${deskPane ? `${deskPane.cols}x${deskPane.rows}` : 'null'}（视口${V_DESK.width}x${V_DESK.height}）`);

  // 基线快照 + 结构行序列（双采样一致 + 结构行判定）。
  // 对比面 = 结构行家族内「同序位逐字 + 边框列位置」，非绝对行序位——pane 内容
  // 在基线后仍可向下增长（prompt 重绘等，首跑实证 16 条回显使绝对序位全位移），
  // 内容增长只从尾部吞 blank 结构行（幸存前缀序位不变），而 shared 压缩翻紧凑
  // 布局会整体改写边框列 + 全部行文本（D-07 判别面守恒）。
  const s1 = await termRows(pageD);
  await sleep(1200); // 跨秒级采样窗（探针实证本 config 无时钟行，此窗为保守护栏）
  const s2 = await termRows(pageD);
  const struct0 = s2.filter((r, i) => i < s1.length && r === s1[i] && structuralRow(r));
  const borderCols0 = [...new Set(struct0.map((r) => r.indexOf('│')))];
  t0.ok(struct0.length >= 10 && borderCols0.length === 1,
    `基线结构行序列非平凡（≥10 行含边框结构行且边框列唯一——空/小集合会使逐字比对假绿）`,
    `结构行=${struct0.length}/${s2.length} 边框列=${borderCols0.join(',')}`);
  await pageD.screenshot({ path: 'screenshots/p14-t0-desktop.png' });

  // ── 移动 tab attach（竖屏小屏）──
  ctxM = await browser.newContext({ viewport: V_MOB_PORTRAIT, extraHTTPHeaders: { Authorization: AUTH_HEADER } });
  const pageM = await ctxM.newPage();
  // 假绿防线：锚定桌面端键入的 stty 标记——移动端渲染出 pane 共享内容 = herdr 全量
  // 首帧到达（探针实证移动端渲染落定需数秒；过早断言桌面恒不变 = 没测到任何东西）
  await openTab(pageM, /:\d+ \d+/, '移动 tab（pane 共享内容锚定）', 25000);
  const mobRender0 = await renderRowCount(pageM);
  t1.ok(mobRender0 > 0, '移动 tab attach 就绪：herdr 全量首帧到达（桌面键入的 stty 标记在移动端 buffer 可见——pane 内容共享证据）',
    `渲染行数=${mobRender0}（视口${V_MOB_PORTRAIT.width}x${V_MOB_PORTRAIT.height}）`);

  // T0 自证核心：第二 tab attach 后 herdr client 进程 1→2（两 pid 不等——per-client
  // 每客户端独立 spawn；shared 模式下第二 tab 复用同进程，计数恒 1）
  pids = await herdrClientPids();
  t0.ok(pids.length === 2 && pids[0] !== pids[1],
    'per-client 模式自证：移动 tab attach 后 herdr client 进程增至 2 且两 pid 不等（每客户端独立进程；shared 下应保持 1）',
    `计数=${pids.length} pid不等=${pids.length === 2 && pids[0] !== pids[1]}`);
  await pageM.screenshot({ path: 'screenshots/p14-t0-mobile.png' });

  // ── T1：小屏 attach 后大屏结构行逐字不变 ──
  const readyM1 = await stableRows(pageM);
  t1.ok(readyM1.stable, '移动端渲染落定（行文本双采样稳定——herdr 前台重排完成后再断言桌面）', `稳定=${readyM1.stable}`);
  await sleep(800); // 吸收窗（非精确时点断言——迟到的增量帧护拦）
  const rows1 = await termRows(pageD);
  const struct1 = rows1.filter(structuralRow);
  const borderCols1 = [...new Set(struct1.map((r) => r.indexOf('│')))];
  const nCmp1 = Math.min(struct0.length, struct1.length);
  let diff1 = 0, firstDiff1 = -1;
  for (let k = 0; k < nCmp1; k++) {
    if (struct1[k] !== struct0[k]) { diff1++; if (firstDiff1 < 0) firstDiff1 = k; }
  }
  t1.ok(diff1 === 0 && borderCols1.join(',') === borderCols0.join(','),
    '小屏 tab attach 后大屏 tab 面板边框结构行同序位逐字一致（边框列位置零漂移——未被移动端几何压缩；旧 shared 语义下整屏翻紧凑布局，结构行必变）',
    `结构行=${struct1.length}/${struct0.length} 序位不一致=${diff1} 首差序位=${firstDiff1} 边框列=${borderCols1.join(',')}/${borderCols0.join(',')}`);
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
  await sleep(800); // 吸收窗
  const rows2 = await termRows(pageD);
  const struct2 = rows2.filter(structuralRow);
  const borderCols2 = [...new Set(struct2.map((r) => r.indexOf('│')))];
  const nCmp2 = Math.min(struct0.length, struct2.length);
  let diff2 = 0, firstDiff2 = -1;
  for (let k = 0; k < nCmp2; k++) {
    if (struct2[k] !== struct0[k]) { diff2++; if (firstDiff2 < 0) firstDiff2 = k; }
  }
  t2.ok(diff2 === 0 && borderCols2.join(',') === borderCols0.join(','),
    '小屏 tab 转屏后大屏 tab 面板边框结构行仍同序位逐字一致（移动端 resize 不压缩桌面端观感——D-07）',
    `结构行=${struct2.length}/${struct0.length} 序位不一致=${diff2} 首差序位=${firstDiff2} 边框列=${borderCols2.join(',')}/${borderCols0.join(',')}`);
  await pageD.screenshot({ path: 'screenshots/p14-t2-desktop.png' });
  await pageM.screenshot({ path: 'screenshots/p14-t2-mobile.png' });

  // ── T0 尾段：herdr 会话共享对照（键入放在 T1/T2 断言之后——pane 新输出只落
  // 内容行，目标行不受影响；移动端此时为前台，stty 回读即移动派生几何）──
  const pc = await paneEcho(pageM, '$fish_pid $(stty size)', '(\\d+) (\\d+) (\\d+)');
  const pidC = pc?.[1] ?? null;
  const mobPane = pc ? { rows: +pc[2], cols: +pc[3] } : null;
  t0.ok(pidC !== null && pidC === pidA,
    'herdr 会话共享对照：移动 tab 键入读回的 pane shell pid 与桌面 tab 相同（双 tab 经 herdr 汇聚同一会话——driving 场景前提）',
    `同pid=${pidA !== null && pidC === pidA}`);
  t0.ok(mobPane !== null && deskPane !== null && deskPane.cols > mobPane.cols,
    '双 tab herdr 几何显著不同（桌面全量派生宽于移动派生——场景有效性，A5 材料）',
    `桌面pane=${deskPane ? deskPane.cols + 'x' + deskPane.rows : 'null'} 移动pane=${mobPane ? mobPane.cols + 'x' + mobPane.rows : 'null'}`);

  await ctxD.close().catch(() => {});
  await ctxM.close().catch(() => {});
} catch (e) {
  console.log(`  FAIL  场景异常: ${e.message}`);
  results.push(false);
} finally {
  // 清理序列（D-09 + 14-08 实证语义）：browser → 转发器 → wesh → herdr session
  // stop + delete（wesh 死后 herdr server 存活是特性，必须显式 stop；delete 清册）。
  // 任一步失败不阻断后续清理步。
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
