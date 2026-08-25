// Phase 6 UAT 浏览器实测层（Playwright 真实 Chromium，Windows GUI 侧运行）——
// 覆盖 .planning/phases/06-session-lifecycle/06-UAT.md 全部六项人工清单：
//   T1 断网 30s 恢复自动重连（面板三件套逐字 + 退避观测 + 同 pid 接回 + 清屏不回放）
//   T2 重连清屏与 vim 重绘观感（SIGWINCH 秒级重绘 + 截图留档）
//   T3 Reconnect now 手动跳过（倒计时未完即 attempt）
//   T4 Session ended 双形态（exit 42 双端逐字 + SIGHUP 大写信号名 + 进程退出码）
//   T5 --once 第二客户端 503（Server is full 三件套逐字 + 断开退出 255）
//   T6 owner 断线重连不恢复写权限（D-06 递补语义，双转发端口独立断连）
//
// 断网模拟：本机 TCP 转发器 kill/restore（双端 RST → 浏览器合成 1006）；
// 恢复快路径经 window.dispatchEvent(new Event('online')) 合成——转发器恢复对 OS 网络栈
// 不可见，该调用与真实断网恢复时浏览器派发的 online 事件命中同一监听器（phase06-dom D8
// 同形态）。真实 OS 网卡栈时序/像素观感属平台豁免（CODEBUDDY.md 豁免条款），截图留档。
//
// 红线（phase04.mjs 纪律沿用）：凭据/share token 值只作断言材料，永不进 detail/控制台输出。
//
// 运行：pnpm -C web/uat/pw install && WESH_UAT_SSH=user@host pnpm -C web/uat/pw uat:06
// （双机模型与全部环境变量见 web/uat/pw/README.md）
import { Check, sleep } from './lib/check.mjs';
import { Forwarder } from './lib/forwarder.mjs';
import { ensureRunSh, ensureNormal, startWesh, exitStatus, ssh, TARGET_HOST, TARGET_PORT } from './lib/server.mjs';
import * as B from './lib/browser.mjs';

const PORT_BASE = parseInt(process.env.WESH_UAT_PORT_BASE || '17681', 10);
const NORMAL = () => `--writable --credential ${B.CRED} --insecure-http -- bash`;
const url = (port) => `http://127.0.0.1:${port}/`;

// ── T1 ──────────────────────────────────────────────────────────
async function t1() {
  const C = new Check(1, '断网 30s 恢复自动重连');
  await ensureRunSh();
  await ensureNormal(B.CRED);
  const fwd = new Forwarder(PORT_BASE, TARGET_HOST, TARGET_PORT);
  await fwd.start();
  const browser = await B.launch();
  try {
    const ctx = await B.authedContext(browser);
    const page = await ctx.newPage();
    await B.openSession(page, url(PORT_BASE));

    const pid1 = await B.getShellPid(page);
    C.ok(!!pid1, 'shell pid 获取', `pid=${pid1}`);
    const before = B.marker('BEFORE');
    await B.runCmd(page, `echo ${before}`);
    await B.waitTermText(page, new RegExp(before));

    const tKill = Date.now();
    fwd.killNet();
    const p = await B.waitPanel(page, 12000);
    const appearMs = Date.now() - tKill;
    C.ok(p.title === 'Reconnecting', '面板标题 Reconnecting', p.title);
    C.ok(/^The connection was lost\. Retrying in \d+s \(attempt \d+\)\.$/.test(p.body), '等待期正文逐字模板', p.body);
    C.ok(p.hint.startsWith('If the server has exited, restart it from your shell. To skip the wait,'), '提示行前缀逐字', p.hint.slice(0, 80));
    C.ok(p.action === 'Reconnect now', '动作链接 Reconnect now', String(p.action));
    C.ok(appearMs <= 8000, '数秒内出现面板', `${appearMs}ms`);

    const samples = [];
    const tEnd = Date.now() + 30000;
    while (Date.now() < tEnd) {
      const cur = await B.panel(page);
      if (!cur.hidden) {
        let m = cur.body.match(/^The connection was lost\. Retrying in (\d+)s \(attempt (\d+)\)\.$/);
        if (m) samples.push({ t: Date.now(), kind: 'wait', s: +m[1], n: +m[2] });
        else if ((m = cur.body.match(/^The connection was lost\. Retrying now \(attempt (\d+)\)\.\.\.$/))) samples.push({ t: Date.now(), kind: 'now', n: +m[1] });
      }
      await sleep(500);
    }
    const waits = samples.filter((x) => x.kind === 'wait');
    const attempts = [...new Set(samples.map((x) => x.n))].sort((a, b) => a - b);
    C.ok(attempts.length >= 3, '30s 内 attempt 计数递增（≥3 个不同 attempt）', attempts.join(','));
    const expectedDelay = (n) => Math.min(2 ** (n - 1), 30);
    const badDelay = waits.filter((w) => w.s > expectedDelay(w.n));
    C.ok(badDelay.length === 0, '倒计时初值服从 1s×2 封顶 30s', badDelay.length ? JSON.stringify(badDelay[0]) : `attempts=${attempts.join(',')}`);
    C.ok(attempts.every((v, i) => i === 0 || v > attempts[i - 1]), 'attempt 序号单调递增');
    let countdownOk = false;
    for (const n of attempts) {
      const seq = waits.filter((w) => w.n === n).map((w) => w.s);
      if (seq.length >= 2 && seq[0] >= 2) {
        countdownOk = seq.every((v, i) => i === 0 || v <= seq[i - 1]);
        if (countdownOk) break;
      }
    }
    C.ok(countdownOk, '同 attempt 内 1Hz 倒计时递减');

    fwd.restore();
    await B.fireOnline(page);
    const hiddenMs = await B.waitPanelHidden(page, 6000);
    C.ok(hiddenMs <= 5000, '恢复后 5s 内自动接回（online 快路径）', `${hiddenMs}ms`);

    let markerGone = false;
    for (let i = 0; i < 20; i++) {
      if (!(await B.termText(page)).includes(before)) { markerGone = true; break; }
      await sleep(250);
    }
    C.ok(markerGone, '接回清屏——断前现场不残留不回放');

    const pid2 = await B.getShellPid(page);
    C.ok(pid1 === pid2, '接回原会话（同一 shell 进程）', `pid ${pid1} → ${pid2}`);
    await ctx.close();
  } finally {
    await browser.close();
    await fwd.stop();
  }
  return C.summary();
}

// ── T2 ──────────────────────────────────────────────────────────
async function t2() {
  const C = new Check(2, '重连清屏与 vim 重绘');
  await ensureNormal(B.CRED);
  const nonce = B.marker('VR');
  const vimFile = `/tmp/wesh-uat/vim-${nonce}.txt`;
  await ssh(`bash -lc 'printf "VIMREDRAW_${nonce}\\nsecond line\\n" > ${vimFile}'`);

  const fwd = new Forwarder(PORT_BASE + 1, TARGET_HOST, TARGET_PORT);
  await fwd.start();
  const browser = await B.launch();
  try {
    const ctx = await B.authedContext(browser);
    const page = await ctx.newPage();
    await B.openSession(page, url(PORT_BASE + 1));

    const pre = B.marker('PRE');
    await B.runCmd(page, `echo ${pre}`);
    await B.waitTermText(page, new RegExp(pre));

    await B.runCmd(page, `vim -u NONE -n ${vimFile}`);
    await B.waitTermText(page, new RegExp(`VIMREDRAW_${nonce}`), 10000);
    C.ok(true, 'vim 打开并渲染文件内容');

    fwd.killNet();
    const p = await B.waitPanel(page, 12000);
    C.ok(p.title === 'Reconnecting', '断网后 Reconnecting 面板出现');

    fwd.restore();
    await B.fireOnline(page);
    const hiddenMs = await B.waitPanelHidden(page, 6000);
    C.ok(hiddenMs <= 5000, '恢复后 5s 内面板消失', `${hiddenMs}ms`);

    let sawBlank = false;
    let redrawn = false;
    const t0 = Date.now();
    while (Date.now() - t0 < 6000) {
      const t = await B.termText(page);
      if (!t.includes(`VIMREDRAW_${nonce}`) && t.trim().length <= 2) sawBlank = true;
      if (t.includes(`VIMREDRAW_${nonce}`)) { redrawn = true; break; }
      await sleep(100);
    }
    C.ok(redrawn, 'vim 经 SIGWINCH 秒级重绘出完整画面');
    if (sawBlank) C.ok(true, '重绘前观测到清屏瞬态（旧画面不残留）');
    else C.note('清屏瞬态未采样到（重绘过快），以截图与终态为准——终态无旧内容残留');

    C.ok(!(await B.termText(page)).includes(pre), '断前 shell 现场不回放不叠影');

    await page.screenshot({ path: `screenshots/t2-vim-redraw-${nonce}.png` });
    C.note(`截图留档 screenshots/t2-vim-redraw-${nonce}.png 供像素观感人工复核`);

    await page.keyboard.press('Escape');
    await page.keyboard.type(':q!');
    await page.keyboard.press('Enter');
    try {
      await B.waitForPrompt(page, 8000);
      C.ok(true, 'vim 退出恢复 shell 提示符（现场清理）');
    } catch {
      C.note('vim 退出后提示符未在 8s 内恢复——后续测试将重启服务端');
    }
    await ctx.close();
  } finally {
    await browser.close();
    await fwd.stop();
    await ssh(`rm -f ${vimFile}`).catch(() => {});
  }
  return C.summary();
}

// ── T3 ──────────────────────────────────────────────────────────
async function t3() {
  const C = new Check(3, 'Reconnect now 手动跳过');
  await ensureNormal(B.CRED);
  const fwd = new Forwarder(PORT_BASE + 2, TARGET_HOST, TARGET_PORT);
  await fwd.start();
  const browser = await B.launch();
  try {
    const ctx = await B.authedContext(browser);
    const page = await ctx.newPage();
    await B.openSession(page, url(PORT_BASE + 2));

    fwd.killNet();
    const p0 = await B.waitPanel(page, 12000);
    C.ok(p0.title === 'Reconnecting', '断网后 Reconnecting 面板出现');

    let s = 0, n = 0;
    const t0 = Date.now();
    while (Date.now() - t0 < 20000) {
      const p = await B.panel(page);
      const m = p.body.match(/^The connection was lost\. Retrying in (\d+)s \(attempt (\d+)\)\.$/);
      if (m && +m[1] >= 3) { s = +m[1]; n = +m[2]; break; }
      await sleep(200);
    }
    C.ok(s >= 3, '捕获到倒计时 ≥3s 的等待态', s >= 3 ? `Retrying in ${s}s (attempt ${n})` : '未捕获');

    fwd.restore();
    await page.click('#status-hint a');
    const hiddenMs = await B.waitPanelHidden(page, Math.max(6000, s * 1000 + 2000));
    C.ok(hiddenMs < s * 1000, '点击后立即重连（远小于剩余倒计时）', `点击→接回 ${hiddenMs}ms ≪ 剩余 ${s}s`);

    const mk = B.marker('T3OK');
    await B.runCmd(page, `echo ${mk}`);
    await B.waitTermText(page, new RegExp(mk), 8000);
    C.ok(true, '接回后终端可输入（会话活性）');
    await ctx.close();
  } finally {
    await browser.close();
    await fwd.stop();
  }
  return C.summary();
}

// ── T4 ──────────────────────────────────────────────────────────
async function t4() {
  const C = new Check(4, 'Session ended 面板（exit 42 / SIGHUP）');
  // 必须经 run.sh 包装启动（exit.status 捕获），不能复用未包装实例
  await startWesh(NORMAL());
  const fwd = new Forwarder(PORT_BASE + 3, TARGET_HOST, TARGET_PORT);
  await fwd.start();
  const browser = await B.launch();
  try {
    const ctxA = await B.authedContext(browser);
    const ctxB = await B.authedContext(browser);
    const pageA = await ctxA.newPage();
    const pageB = await ctxB.newPage();
    await B.openSession(pageA, url(PORT_BASE + 3));
    await B.openSession(pageB, url(PORT_BASE + 3));

    await B.runCmd(pageA, 'exit 42');
    const [pA, pB] = await Promise.all([B.waitPanel(pageA, 15000), B.waitPanel(pageB, 15000)]);
    C.ok(pA.title === 'Session ended', 'A 端 Session ended 标题', pA.title);
    C.ok(pA.body === 'The process exited with code 42.', 'A 端正文逐字（退出码人话）', pA.body);
    C.ok(pB.title === 'Session ended', 'B 端 Session ended 标题', pB.title);
    C.ok(pB.body === 'The process exited with code 42.', 'B 端正文逐字（双端一致广播）', pB.body);
    const codeA = await exitStatus(20000);
    C.ok(codeA === 42, 'wesh 进程退出码 42', `exit=${codeA}`);
    await ctxA.close();
    await ctxB.close();

    await startWesh(NORMAL());
    const page = await (await B.authedContext(browser)).newPage();
    await B.openSession(page, url(PORT_BASE + 3));

    await B.runCmd(page, 'kill -HUP $$');
    const p = await B.waitPanel(page, 15000);
    C.ok(p.title === 'Session ended', '信号形态 Session ended 标题', p.title);
    C.ok(p.body === 'The process was killed by signal SIGHUP.', '正文逐字（大写信号名）', p.body);
    const codeB = await exitStatus(20000);
    C.ok(codeB === 255, 'wesh 进程退出状态 255', `exit=${codeB}`);
    await page.context().close();
  } finally {
    await browser.close();
    await fwd.stop();
  }
  return C.summary();
}

// ── T5 ──────────────────────────────────────────────────────────
async function t5() {
  const C = new Check(5, '--once 第二客户端 Server is full');
  await startWesh(`--once --writable --credential ${B.CRED} --insecure-http -- bash`);
  const fwd = new Forwarder(PORT_BASE + 5, TARGET_HOST, TARGET_PORT);
  await fwd.start();
  const browser = await B.launch();
  try {
    const ctxA = await B.authedContext(browser);
    const pageA = await ctxA.newPage();
    await B.openSession(pageA, url(PORT_BASE + 5));
    C.ok(true, '窗口 A 正常进入（唯一槽位）');

    const ctxB = await B.authedContext(browser);
    const pageB = await ctxB.newPage();
    const resp = await pageB.goto(url(PORT_BASE + 5), { waitUntil: 'domcontentloaded', timeout: 20000 });
    C.ok(resp.status() === 200, 'B 页面 HTML 加载（attach 闸在页内）', `HTTP ${resp.status()}`);
    const pB = await B.waitPanel(pageB, 15000);
    C.ok(pB.title === 'Server is full', 'B 端 Server is full 标题', pB.title);
    C.ok(pB.body === 'The server has reached its maximum number of attached clients.', 'B 端正文逐字（容量语义）', pB.body);
    C.ok(pB.hint.startsWith('Wait for a slot to free up, then'), 'B 端提示行（等槽位释放）', pB.hint.slice(0, 60));

    const mk = B.marker('T5A');
    await B.runCmd(pageA, `echo ${mk}`);
    await B.waitTermText(pageA, new RegExp(mk), 8000);
    C.ok(true, 'A 端会话活性不受 B 影响');

    await pageA.close();
    const code = await exitStatus(25000);
    C.ok(code === 255, '唯一客户端断开后 wesh 退出状态 255', `exit=${code}`);
    await ctxB.close();
  } finally {
    await browser.close();
    await fwd.stop();
  }
  return C.summary();
}

// ── T6 ──────────────────────────────────────────────────────────
async function t6() {
  const C = new Check(6, 'owner 断线重连不恢复写权限');
  await startWesh(NORMAL());
  const fwdA = new Forwarder(PORT_BASE + 6, TARGET_HOST, TARGET_PORT);
  const fwdB = new Forwarder(PORT_BASE + 7, TARGET_HOST, TARGET_PORT);
  await fwdA.start();
  await fwdB.start();
  const browser = await B.launch();
  try {
    const ctxA = await B.authedContext(browser);
    const pageA = await ctxA.newPage();
    await B.openSession(pageA, url(PORT_BASE + 6));
    const titleA0 = await B.titleOf(pageA);
    C.ok(!titleA0.startsWith('[ro] '), 'A 首连为 owner（无 [ro] 前缀）', titleA0);

    const ctxB = await B.authedContext(browser);
    const pageB = await ctxB.newPage();
    await B.openSession(pageB, url(PORT_BASE + 7));
    const titleB0 = await B.waitTitle(pageB, /^\[ro\] /, 12000);
    C.ok(true, 'B 第二端降级旁观（[ro] 前缀）', titleB0);
    C.ok(!(await B.titleOf(pageA)).startsWith('[ro] '), 'B 进入后 A 仍 owner', await B.titleOf(pageA));

    fwdA.killNet();
    const pA = await B.waitPanel(pageA, 12000);
    C.ok(pA.title === 'Reconnecting', 'A 断网后 Reconnecting 面板出现');

    const titleB1 = await B.waitTitle(pageB, /^(?!\[ro\] ).*/, 10000).catch(() => null);
    C.ok(titleB1 !== null && !titleB1.startsWith('[ro] '), 'A 断开后 B 升格 owner（前缀消失）', String(titleB1));

    fwdA.restore();
    await B.fireOnline(pageA);
    await B.waitPanelHidden(pageA, 6000);
    const titleA2 = await B.waitTitle(pageA, /^\[ro\] /, 12000).catch(() => null);
    C.ok(titleA2 !== null && titleA2.startsWith('[ro] '), 'A 重连后不恢复写权限（[ro] 前缀）', String(titleA2));

    const mk = B.marker('T6B');
    await B.runCmd(pageB, `echo ${mk}`);
    await B.waitTermText(pageB, new RegExp(mk), 8000);
    C.ok(true, 'B 升格后键盘激活可输入');
    const tA = await B.waitTermText(pageA, new RegExp(mk), 8000).catch(() => '');
    C.ok(tA.includes(mk), 'A 以 ro 身份仍接收会话输出');
    await ctxA.close();
    await ctxB.close();
  } finally {
    await browser.close();
    await fwdA.stop();
    await fwdB.stop();
  }
  return C.summary();
}

// ── runner ──────────────────────────────────────────────────────
const ALL = { t1, t2, t3, t4, t5, t6 };
const wanted = process.argv.slice(2).filter((x) => x in ALL);
const list = wanted.length ? wanted : Object.keys(ALL);

const results = [];
for (const id of list) {
  console.log(`\n=== ${id} ===`);
  const t0 = Date.now();
  try {
    const s = await ALL[id]();
    s.elapsedMs = Date.now() - t0;
    results.push(s);
    console.log(`--- ${id} ${s.pass ? 'PASS' : 'FAIL'} (${s.elapsedMs}ms)`);
  } catch (e) {
    results.push({ id, name: id, pass: false, total: 0, failed: [{ label: 'uncaught: ' + e.message }], notes: [], elapsedMs: Date.now() - t0 });
    console.log(`--- ${id} ERROR: ${e.message}`);
  }
}

console.log('\n========== SUMMARY ==========');
for (const r of results) {
  console.log(`${r.pass ? 'PASS' : 'FAIL'}  ${r.id} ${r.name} (${r.total} asserts, ${Math.round(r.elapsedMs / 1000)}s)`);
  for (const f of r.failed ?? []) console.log(`      ✗ ${f.label}${f.extra ? ' — ' + f.extra : ''}`);
}
const allPass = results.every((r) => r.pass);
console.log(`\nOVERALL: ${allPass ? 'ALL PASS' : 'FAILURES PRESENT'}`);
process.exit(allPass ? 0 : 1);
