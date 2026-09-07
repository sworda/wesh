// Phase 14 UAT 一键矩阵 runner（零依赖，Node >= 22 原生 child_process/fs/path）——D-04 落地。
//
// 【矩阵清单】17 项 = 16 项既有脚本基线（13-08 收口闸人工逐跑实测口径）+ phase14.mjs：
//   协议 12 —— phase02/03/04/05/05-dims/06/07/08/09/11/12/13
//   jsdom  4 —— phase04-dom/05-dom/06-dom/12-dom
//   herdr  1 —— phase14（含真实等待，排末位）
// 执行序：协议基线（phase02-09，9 项）→ jsdom（4 项）→ per-client 协议（phase11/12/13，
// 3 项）→ herdr（phase14，末位）。辅助脚本（phase04-t1-width/phase05-flood-driver/
// phase07-b*/phase08-journal/minrepro-*）不入矩阵——16 项（+phase14）即「零遗漏」的
// 定义基准（RESEARCH §Pattern 5）。
//
// 【红线】runner 零断言零解析零 skip 开关——门禁语义归各子脚本既有 exit code 0/1
// （phase13.mjs 尾 process.exit 形态），本脚本只做串行 spawn + 聚合 + 汇总表；自身
// 不新增任何 token/pid 打印面（透传面红线，T-14-22——行前缀仅为脚本名），敏感值
// 自净归各子脚本 assertOutputClean 既有面。矩阵不得以跳过/豁免换绿（skipped 仅各
// 脚本内平台豁免类且带 reason——CODEBUDDY.md §5 口径）。
//
// 【时序纪律】串行执行（脚本间进程清理与端口释放互不干扰）；逐脚本 10min 超时护栏
// （herdr 场景含真实等待——超时 SIGTERM 整个进程组转 FAIL，不阻断后续脚本，T-14-21；
// 2s 宽限后 SIGKILL 兜底）。
//
// 【运行】node web/uat/run-all.mjs [wesh 二进制路径] [脚本名过滤子集...]
//   二进制默认 /tmp/wesh-uat/wesh（缺席自动 go build -o <path> ./cmd/wesh，构建
//   失败打印构建指引退出 1——「先构建后指引」形态）；过滤子集为脚本名（含/不含
//   .mjs 后缀均可，空格分隔多项，按矩阵序执行；未知名 exit 2 防拼写错误静默跑空
//   ——空矩阵不得绿）。传过滤子集但用默认二进制时首位占位 ''：
//   node web/uat/run-all.mjs '' phase11 phase12 phase13（SC2 三脚本复跑形态）。
//   退出码：全绿 0 / 任一失败 1 / 用法错误 2。
//   pw 层不纳入矩阵（双机拓扑硬约束，RESEARCH OQ3 裁决）：Windows 工作站独立执行
//   node web/uat/pw/phase14-pw.mjs——汇总表末尾注记，收口闸人工两段式
//   （Linux runner 全绿 + Windows pw 全绿）。
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url)); // <repo>/web/uat
const ROOT = path.resolve(HERE, '../..'); // 仓库根（go build cwd；子脚本 cwd 同 13-08 逐跑形态）

// 矩阵 17 项（与 web/uat/ ls 逐字核对；协议→jsdom→per-client→herdr 场景序，phase14 末位）
const SCRIPTS = [
  'phase02.mjs', 'phase03.mjs', 'phase04.mjs', 'phase05.mjs', 'phase05-dims.mjs',
  'phase06.mjs', 'phase07.mjs', 'phase08.mjs', 'phase09.mjs',
  'phase04-dom.mjs', 'phase05-dom.mjs', 'phase06-dom.mjs', 'phase12-dom.mjs',
  'phase11.mjs', 'phase12.mjs', 'phase13.mjs',
  'phase14.mjs',
];

const DEFAULT_WESH = '/tmp/wesh-uat/wesh';
const TIMEOUT_MS = 10 * 60 * 1000; // 逐脚本超时护栏（herdr 真实等待上限量级，RESEARCH §Pattern 5）
const norm = (s) => s.replace(/\.mjs$/, '');

// argv[2] 二进制路径（'' 占位回退默认——过滤子集场景）；argv[3:] 过滤子集
const rawWesh = process.argv[2] ?? '';
if (SCRIPTS.includes(rawWesh) || SCRIPTS.includes(`${rawWesh}.mjs`)) {
  // 防混位守卫：脚本名被误传为二进制路径时，自动构建会把产物写到错误位置——显式拦截
  console.error('[run-all] 用法: node web/uat/run-all.mjs [wesh 二进制路径] [脚本名过滤子集...]');
  console.error(`[run-all] 首参 '${rawWesh}' 是矩阵脚本名——二进制路径与过滤子集不可混位（默认二进制占位 ''）`);
  process.exit(2);
}
const WESH = rawWesh || DEFAULT_WESH;

let targets = SCRIPTS;
const FILTERS = process.argv.slice(3).map(norm);
if (FILTERS.length > 0) {
  const wanted = new Set(FILTERS);
  targets = SCRIPTS.filter((s) => wanted.has(norm(s)));
  const unknown = FILTERS.filter((f) => !targets.some((t) => norm(t) === f));
  if (unknown.length > 0) {
    console.error(`[run-all] 未知脚本名: ${unknown.join(' ')}`);
    console.error(`[run-all] 可用: ${SCRIPTS.map(norm).join(' ')}`);
    process.exit(2);
  }
}

// 前置保障：二进制存在性检查——缺席先构建，构建失败明确指引（phase13.mjs 头注释约定形态）
if (!fs.existsSync(WESH)) {
  console.log(`[run-all] 二进制缺席（${WESH}）——尝试构建 go build -o ${WESH} ./cmd/wesh`);
  fs.mkdirSync(path.dirname(WESH), { recursive: true });
  const built = spawnSync('go', ['build', '-o', WESH, './cmd/wesh'], { cwd: ROOT, stdio: 'inherit' });
  if (built.status !== 0 || !fs.existsSync(WESH)) {
    console.error(`[run-all] 构建失败——请在仓库根 ${ROOT} 手动执行: go build -o ${WESH} ./cmd/wesh`);
    process.exit(1);
  }
}

// 逐脚本执行：管道转发 + [脚本名] 行前缀（可读性优先择一形态）；detached 使子树自成
// 进程组，超时 kill(-pgid) 整组收割（挂死脚本 + 其未及自清的 wesh 子进程一并终结）
function runScript(name) {
  return new Promise((resolve) => {
    const t0 = Date.now();
    const tag = norm(name);
    const child = spawn('node', [path.join(HERE, name), WESH], {
      cwd: ROOT, stdio: ['ignore', 'pipe', 'pipe'], detached: true,
    });
    let timedOut = false;
    const attachForward = (stream, write) => {
      let pending = '';
      stream.on('data', (chunk) => {
        const lines = (pending + chunk.toString()).split('\n');
        pending = lines.pop(); // 末段可能是半行，留待下一块或 close 冲刷
        for (const line of lines) write(`[${tag}] ${line}\n`);
      });
      return () => pending;
    };
    const flushOut = attachForward(child.stdout, (l) => process.stdout.write(l));
    const flushErr = attachForward(child.stderr, (l) => process.stderr.write(l));
    const timer = setTimeout(() => {
      timedOut = true;
      console.error(`[${tag}] ⏱ 超时护栏（>10min）触发——SIGTERM 进程组，转 FAIL 不阻断后续脚本`);
      try { process.kill(-child.pid, 'SIGTERM'); } catch { /* 进程组已消亡 */ }
      // 2s 宽限后 SIGKILL 兜底（ESRCH 容忍——组已清则无事发生）
      setTimeout(() => { try { process.kill(-child.pid, 'SIGKILL'); } catch { /* 进程组已消亡 */ } }, 2000).unref();
    }, TIMEOUT_MS);
    child.on('error', (e) => {
      clearTimeout(timer);
      resolve({ name, ok: false, durMs: Date.now() - t0, note: `spawn error: ${e.code ?? e.message}` });
    });
    child.on('close', (code, signal) => {
      clearTimeout(timer);
      const restOut = flushOut();
      if (restOut) process.stdout.write(`[${tag}] ${restOut}\n`);
      const restErr = flushErr();
      if (restErr) process.stderr.write(`[${tag}] ${restErr}\n`);
      const durMs = Date.now() - t0;
      const note = timedOut ? 'TIMEOUT>10min' : signal ? `signal ${signal}` : `exit ${code}`;
      resolve({ name, ok: !timedOut && code === 0, durMs, note });
    });
  });
}

console.log(`[run-all] wesh: ${WESH}`);
console.log(`[run-all] 矩阵 ${targets.length}/${SCRIPTS.length} 项串行: ${targets.map(norm).join(' ')}\n`);
const results = [];
for (const name of targets) {
  console.log(`[run-all] ▶ ${name}`);
  results.push(await runScript(name));
}

// 汇总表：逐脚本 结果/耗时 + 总计 + pw 独立入口注记（OQ3 裁决）+ exit code 门禁
const totalMs = results.reduce((n, r) => n + r.durMs, 0);
const passN = results.filter((r) => r.ok).length;
console.log('\n===== UAT 矩阵汇总 =====');
for (const r of results) {
  console.log(`${r.name.padEnd(18)} ${r.ok ? 'PASS' : 'FAIL'}  ${(r.durMs / 1000).toFixed(1).padStart(7)}s  ${r.note}`);
}
console.log('-------------------------');
console.log(`总计: ${passN}/${results.length} PASS，全程 ${(totalMs / 1000).toFixed(1)}s`);
console.log('注: pw 层独立执行入口（Windows 工作站）: node web/uat/pw/phase14-pw.mjs —— 双机拓扑硬约束不纳入本矩阵（收口闸人工两段式）');
process.exit(passN === results.length ? 0 : 1);
