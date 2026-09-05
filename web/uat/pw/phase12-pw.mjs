// Phase 12 UAT：CR-01 真实浏览器 resize 观感实证（pw 层，Windows 侧 Playwright）。
//
// 定位：12-UAT.md 检查点 1「CR-01 真实浏览器 resize 观感验证」的实测执行载具。
// 逻辑面已由 web/uat/phase12-dom.mjs（jsdom）红→绿 17/17 覆盖（D2e/D2f/D2g + D1）；
// 本载具补的是 jsdom 够不到的那一层——**真实 Chromium 的真实 fit 链路**：真实视口
// 尺寸变化 → xterm fit 插件实测字符度量 → proposeDimensions → 渲染 DOM。jsdom 侧
// 的布局是手工桩（getBoundingClientRect/getComputedStyle/metrics 全伪造），真实
// 浏览器的字体度量、滚动条宽度、DPR 均由引擎给出，二者非同一代码路径。
//
// 拓扑（本目录 README.md 双机模型）：
//   Windows Chromium --127.0.0.1:PORT_BASE--> 本机 TCP 转发器 --LAN--> Linux wesh :7681
// 转发器（非 ssh -L——2026-08-26 教训：ssh -L 隧道在这个环境里被安全代理杀；转发器
// 是纯 LAN 直连，且提供 killNet/restore 供 T5 重连面使用）。
//
// CR-01 判别面（修复前症状：sessionDims 唯一赋值点在 WELCOME 分支，per-client 无
// 'W' 尺寸推送 → 无刷新闭环 → 渲染恒钳在 attach 尺寸 min(fit, attach)）：
//   - 放大：渲染行数/列数必须【跟随】放大后的 fit（不跟随 = 钳在 attach 尺寸）
//   - 缩小：必须【回落】（往复，排除单向偶然）
//   - 回原尺寸：必须【回到基线】（无漂移、无 attach 尺寸残留）
//   - cols 轴：放大后打印长行必须【单行不折】（折行 = 渲染 cols 仍为 attach 值，
//     而 shell 按 PTY 实际 cols 输出字节流 → 折行错位，即 CR-01 用户可见症状）
//
// 模式自证（T1b）：per-client 的语义定义是「每客户端独立会话进程」——两个浏览器页
// 的 shell PID 必须不同。这条断言把「--session-mode per-client 真的生效」从启动
// 参数层面抬升到可观测行为层面，避免整个载具跑在 shared 模式还全绿（假绿）。
//
// 红线（lib/browser.mjs 头注释纪律继承）：凭据/token 值永不进 detail/控制台输出
// （只打状态码/布尔/尺寸数/标记常量）。
import { mkdirSync } from 'node:fs';
import { gunzipSync } from 'node:zlib';
import { Check, sleep } from './lib/check.mjs';
import { Forwarder } from './lib/forwarder.mjs';
import { TARGET_HOST, TARGET_PORT, ssh, ensureRunSh, startWesh, stopWesh } from './lib/server.mjs';
import { launch, CRED, openSession, runCmd, waitTermText, panel, fireOnline } from './lib/browser.mjs';

// 视口阶梯（attach 小 → 放大 → 缩小 → 回基线，往复判别）
const V0 = { width: 800, height: 560 };   // attach 基线
const V_UP = { width: 1600, height: 1000 }; // 放大（cols 约翻倍，长行判别面）
const V_DOWN = { width: 700, height: 420 }; // 缩小（低于基线，回落判别面）

const PORT_BASE = parseInt(process.env.WESH_UAT_PORT_BASE || '17681', 10);
const BASE = `http://127.0.0.1:${PORT_BASE}`;
const AUTH_HEADER = 'Basic ' + Buffer.from(CRED).toString('base64');

const results = [];

// ── 观测通道 ──
// 渲染行数：DOM 渲染器 .xterm-rows 行 div 数恒等于 term.rows（phase05-dom D6a /
// phase12-dom D2e 同一通道，真实 Chromium 同族）。--disable-webgl 由 launch() 强制。
const renderRows = (page) =>
  page.evaluate(() => document.querySelector('.xterm-rows')?.childElementCount ?? -1);

// PTY 侧真实尺寸（服务端直通落定后的权威值）——渲染跟随断言的对照基准
async function sttySize(page) {
  const nonce = 'SZ_' + Math.random().toString(36).slice(2, 6).toUpperCase();
  await runCmd(page, `echo ${nonce}:$(stty size)`);
  const t = await waitTermText(page, new RegExp(`${nonce}:(\\d+) (\\d+)`), 8000);
  const m = t.match(new RegExp(`${nonce}:(\\d+) (\\d+)`));
  return m ? { rows: +m[1], cols: +m[2] } : null;
}

// 当前 shell PID（重连「是否换了新进程」的判别通道）
async function shellPid(page, waitMs = 6000) {
  const nonce = 'PID_' + Math.random().toString(36).slice(2, 8).toUpperCase();
  await runCmd(page, `echo ${nonce}:$$`);
  const t = await waitTermText(page, new RegExp(`${nonce}:(\\d+)`), waitMs);
  return t.match(new RegExp(`${nonce}:(\\d+)`))?.[1] ?? null;
}

async function waitRows(page, want, label, timeout = 5000) {
  const t0 = Date.now();
  for (;;) {
    const r = await renderRows(page);
    if (r === want) return r;
    if (Date.now() - t0 > timeout) throw new Error(`waitRows 超时(${label}): 期望 ${want} 实到 ${r}`);
    await sleep(50);
  }
}

// 渲染行数稳定窗（防抖落定判别：连续 N 次采样同值）——避免断言命中过渡态
async function stableRows(page, ms = 700, interval = 100) {
  let last = -1, same = 0;
  for (let i = 0; i < ms / interval + 10; i++) {
    const r = await renderRows(page);
    same = r === last ? same + 1 : 0;
    last = r;
    if (same >= 3) return r;
    await sleep(interval);
  }
  return last;
}

// T0 前置自检：web/embed.go 对 gzip 客户端直发构建期预压体 dist/index.html.gz，
// 非 gzip 客户端发 dist/index.html——两个实体彼此独立，且 .gz 被 .gitignore 忽略
// （未纳管）。一旦 index.html 更新而 .gz 未随构建重新生成，全部真实浏览器
// （恒带 Accept-Encoding: gzip）将拿到旧包，前端修复被静默回退。本断言比对两
// 条通道解压后的字节，把这整类「预压产物陈旧」挡在观感断言之前——否则本载具
// 会在旧包上跑出全绿（2026-09-05 Phase 12 UAT 实证教训）。
async function servedBodies(browser) {
  const ctx = await browser.newContext();
  const get = async (enc) => {
    const r = await ctx.request.get(`${BASE}/`, {
      headers: { Authorization: AUTH_HEADER, 'Accept-Encoding': enc },
    });
    let buf = await r.body();
    // Playwright 会自动解压但仍保留 content-encoding 头——解压失败即视作已解压
    //（只比对解压后的产物字节，两通道是否走压缩旁路不影响判别）
    if ((r.headers()['content-encoding'] ?? '').includes('gzip')) {
      try { buf = gunzipSync(buf); } catch { /* 已解压，原样使用 */ }
    }
    return buf;
  };
  try {
    return { gz: await get('gzip'), plain: await get('identity') };
  } finally {
    await ctx.close();
  }
}

const t0 = new Check('P12-T0', '伺服自检：gzip 预压通道与明文通道返回同一份前端产物');
const t1 = new Check('P12-T1', 'attach 基线：per-client 模式生效 + 渲染尺寸 == PTY 尺寸');
const t2 = new Check('P12-T2', '放大：渲染行数/列数跟随 fit（CR-01 判别面——不钳在 attach 尺寸）');
const t3 = new Check('P12-T3', '放大后长行单行不折（cols 轴跟随，无折行错位）');
const t4 = new Check('P12-T4', '缩小 + 回基线：渲染回落且无漂移（往复判别）');
const t5 = new Check('P12-T5', '重连 reset 清残影：旧 normal buffer 内容不复活（per-client 新进程）');

mkdirSync('screenshots', { recursive: true });
const fwd = new Forwarder(PORT_BASE, TARGET_HOST, TARGET_PORT);
let browser;
try {
  // ── Linux 侧：per-client wesh（LAN 直连可达；凭据一次性测试凭据）──
  await ensureRunSh();
  await startWesh(
    `--session-mode per-client --writable --insecure-http --credential ${CRED} -- bash --norc --noprofile`,
  );
  await fwd.start();

  browser = await launch();
  // T0：两通道产物一致性（陈旧预压体挡在观感断言之前）
  {
    const { gz, plain } = await servedBodies(browser);
    t0.ok(gz.length > 0 && plain.length > 0, '两条通道均取到文档体',
      `gzip=${gz.length}B 明文=${plain.length}B`);
    t0.ok(Buffer.compare(gz, plain) === 0,
      'gzip 预压体与明文产物逐字节一致（预压通道未回送陈旧包）',
      `一致=${Buffer.compare(gz, plain) === 0} 差=${gz.length - plain.length}B`);
  }

  const ctx = await browser.newContext({
    viewport: V0,
    extraHTTPHeaders: { Authorization: AUTH_HEADER },
  });
  const page = await ctx.newPage();
  await openSession(page, `${BASE}/`);
  await page.screenshot({ path: 'screenshots/p12-attach.png' });

  // ── T1：attach 基线 ──
  const rows0 = await stableRows(page);
  const pty0 = await sttySize(page);
  t1.ok(rows0 > 0 && pty0 !== null, 'attach 后终端就绪（渲染行数可读 + PTY stty 可读）',
    `渲染行数=${rows0} PTY=${pty0 ? `${pty0.rows}x${pty0.cols}` : 'null'}`);
  t1.ok(rows0 === pty0?.rows, 'attach 基线：渲染行数 == PTY 行数（恒等式在 attach 点成立）',
    `渲染=${rows0} PTY=${pty0?.rows}`);

  // T1b 模式自证：per-client = 每客户端独立会话进程 → 第二页 shell PID 必须不同。
  // 若这条不成立，整个载具是跑在 shared 模式下的假绿（T2/T3 会失去意义）。
  const pidA = await shellPid(page);
  const page2 = await ctx.newPage();
  await openSession(page2, `${BASE}/`);
  const pidB = await shellPid(page2);
  t1.ok(pidA !== null && pidB !== null && pidA !== pidB,
    'per-client 模式自证：两浏览器页 shell PID 不同（每客户端独立会话进程；shared 下应相同）',
    `pidA≠pidB=${pidA !== pidB}`);
  await page2.close();

  // ── T2：放大 ──
  await page.setViewportSize(V_UP);
  let rowsUp = -1;
  try {
    await waitRows(page, -1, '放大后渲染行数变化（占位，交由稳定窗取实际值）', 1);
  } catch { /* 预期抛（want=-1 不可达），仅为让出一次事件循环 */ }
  rowsUp = await stableRows(page, 1400);
  const ptyUp = await sttySize(page);
  t2.ok(rowsUp > rows0, '放大后渲染行数【跟随增长】——未被钳在 attach 尺寸（CR-01 修复前恒 rows0）',
    `attach=${rows0} 放大后=${rowsUp}`);
  t2.ok(rowsUp === ptyUp?.rows, '放大后渲染行数 == PTY 行数（sessionDims 恒等式：min(fit,fit)=fit）',
    `渲染=${rowsUp} PTY=${ptyUp?.rows}`);
  t2.ok(ptyUp !== null && ptyUp.cols > pty0.cols, '放大后 PTY cols 增长（服务端 resize 直通生效）',
    `attach=${pty0.cols} 放大后=${ptyUp?.cols}`);
  await page.screenshot({ path: 'screenshots/p12-enlarged.png' });

  // ── T3：cols 轴长行不折 ──
  // L 取「放大后 cols − 20」：需显著大于 attach cols，否则不具判别力（折/不折都过）
  const L = (ptyUp?.cols ?? 0) - 20;
  t3.ok(L > pty0.cols + 10, `长行长度具判别力（L=${L} > attach cols ${pty0.cols} + 10）`,
    `L=${L} attachCols=${pty0.cols}`);
  await runCmd(page, `printf '${'A'.repeat(L)}\\n'`);
  let single = false;
  for (let i = 0; i < 40; i++) {
    single = await page.evaluate(
      (s) => [...document.querySelectorAll('.xterm-rows > div')].some((r) => r.textContent.trimEnd() === s),
      'A'.repeat(L),
    );
    if (single) break;
    await sleep(100);
  }
  t3.ok(single, `放大后 ${L} 字符长行【单行在场】——渲染 cols 已跟随（钳在 attach ${pty0.cols} cols 时会折为两行）`,
    `单行在场=${single}`);
  await page.screenshot({ path: 'screenshots/p12-longline.png' });

  // ── T4：缩小 → 回落；回基线 → 无漂移 ──
  await page.setViewportSize(V_DOWN);
  const rowsDown = await stableRows(page, 1400);
  const ptyDown = await sttySize(page);
  t4.ok(rowsDown < rowsUp, '缩小后渲染行数【回落】（往复，非单向偶然）',
    `放大后=${rowsUp} 缩小后=${rowsDown}`);
  t4.ok(rowsDown === ptyDown?.rows, '缩小后渲染行数 == PTY 行数', `渲染=${rowsDown} PTY=${ptyDown?.rows}`);
  await page.setViewportSize(V0);
  const rowsBack = await stableRows(page, 1400);
  const ptyBack = await sttySize(page);
  t4.ok(rowsBack === rows0, '回到 attach 视口后渲染行数【回到基线】——无漂移、无 attach 尺寸残留',
    `基线=${rows0} 回归=${rowsBack}`);
  t4.ok(rowsBack === ptyBack?.rows, '回归后渲染行数 == PTY 行数', `渲染=${rowsBack} PTY=${ptyBack?.rows}`);
  await page.screenshot({ path: 'screenshots/p12-back.png' });

  // ── T5：重连 reset 清残影 ──
  // 与 phase12-dom D1 同构链路，但驱动源是【真实断网 RST】而非合成 close 事件：
  // 旧会话 normal buffer 写残影 → 1049h 进 alt screen（残影藏身处，clear() 不触达）
  // → 转发器 killNet（在飞连接双端 RST → 浏览器 WS 合成 1006）→ restore + online
  // → 退避重连 → per-client = 全新进程 + 新 WELCOME(session:"per-client") → reset
  // → 新会话 1049l 弹回 → 断言旧 normal buffer 残影不复活。
  await runCmd(page, 'echo NORMGHOST8');
  await waitTermText(page, /NORMGHOST8/, 8000);
  await runCmd(page, "printf '\\033[?1049h'");
  for (let i = 0; i < 40 && (await page.evaluate(() => document.querySelector('.xterm-rows')?.textContent ?? '')).includes('NORMGHOST8'); i++) await sleep(100);
  await runCmd(page, 'echo ALTLIVE8');
  await waitTermText(page, /ALTLIVE8/, 8000);
  const pidOld = await shellPid(page);
  t5.ok(pidOld !== null, 'alt screen 场景就位（残影藏于 normal buffer + 旧 shell PID 已取）', '1049h 已进 alt screen');

  fwd.killNet();
  await sleep(500);
  fwd.restore();
  await fireOnline(page);

  // 重连判别：等到 shell PID 变化（新进程）——不依赖 reset 是否发生，独立通道
  let pidNew = null;
  for (let i = 0; i < 15 && pidNew === null; i++) {
    const p = await shellPid(page, 2000).catch(() => null);
    if (p !== null && p !== pidOld) pidNew = p;
    else await sleep(800);
  }
  t5.ok(pidNew !== null && pidNew !== pidOld, 'killNet 后自动重连且换到新会话进程（per-client 语义）',
    `pid 变化=${pidNew !== null && pidNew !== pidOld}`);

  // 新会话内 1049l 弹回：reset 已生效 → 本就在干净 normal buffer；未 reset →
  // 仍停在 alt screen，弹回复活旧 normal buffer 残影（= 探针 Case A 症状）
  await runCmd(page, "printf '\\033[?1049l'; echo EXITED8");
  await waitTermText(page, /EXITED8/, 10000);
  const txt = await page.evaluate(() => document.querySelector('.xterm-rows')?.textContent ?? '');
  t5.ok(!txt.includes('NORMGHOST8'), 'reset 证据一：1049l 弹回后旧 normal buffer 残影（NORMGHOST8）不复活',
    `残影在场=${txt.includes('NORMGHOST8')}`);
  t5.ok(!txt.includes('ALTLIVE8'), 'reset 证据二：旧 alt buffer 内容（ALTLIVE8）不残留',
    `alt 内容在场=${txt.includes('ALTLIVE8')}`);
  t5.ok(txt.includes('EXITED8'), 'reset 未吞新输出：新会话标记 EXITED8 在场', `新标记在场=${txt.includes('EXITED8')}`);
  const p = await panel(page);
  t5.ok(p.hidden, '重连 reset 静默零新面板（D-10）', `面板隐藏=${p.hidden}`);
  await page.screenshot({ path: 'screenshots/p12-reconnect.png' });

  await ctx.close();
} catch (e) {
  console.log(`  FAIL  场景异常: ${e.message}`);
  results.push(false);
} finally {
  if (browser) await browser.close().catch(() => {});
  await fwd.stop().catch(() => {});
  await stopWesh().catch(() => {});
}

for (const t of [t0, t1, t2, t3, t4, t5]) {
  const s = t.summary();
  if (s.total > 0) results.push(s.pass);
}
const failed = results.filter((r) => !r).length;
console.log(`\n结果: ${results.length - failed}/${results.length} 项通过`);
process.exit(failed ? 1 : 0);
