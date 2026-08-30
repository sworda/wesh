// Phase 09 UAT：Caddy 反代双机全链实证（09-08 D-15；G-07-2 nginx 套路的 Caddy 版——
// Linux 侧 Caddy+wesh 生命周期由同目录配套 .sh 控制脚本经 ssh 管理）。
// 拓扑（见本目录 README.md 双机模型）：Linux 侧一次性 Caddy（/tmp/wesh-uat/caddy，
// LAN 绑定 0.0.0.0:10014 reverse_proxy → loopback wesh :17682，凭据经 WESH_CREDENTIAL
// env 递交——setup 时由本脚本经 ssh stdin 传入，不进 argv/ps 可见面）；Windows 侧
// Playwright Chromium 直连 Linux 侧 LAN IP（端口与 a2 的 10013 错开；连通性以安全组
// 放通为前提，setup 前置检查）。
// Caddy 断言面差异（Pitfall 6，与 nginx 配方互抄必错）：reverse_proxy 默认原样透传
// Host（wesh Origin 同源校验天然过——ctl 侧不配任何 Host 改写行）；WS upgrade 内建
// 自动处理；hijack 后无默认 WS idle 超时（t3 65s 空闲窗实证）。
// 认证形态（与 a2 的关键差异）：a2 的 401 来自 nginx auth_basic 须 httpCredentials
// （WS 握手同栈被 nginx 拦截）；本 rig 的 Caddy 无认证层、wesh /ws 不收 HTTP 级认证，
// 故走 authedContext 预置 Authorization 避开 wesh 401→recordFail→429 节流链
// （lib/browser.mjs 头注释纪律——httpCredentials 的 Chromium 即时重试会撞 1s 窗口）。
// 红线：凭据值只作构造材料，永不进 detail/控制台输出（只打状态码/布尔/文案常量）。
import { execSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { Check, sleep } from './lib/check.mjs';
import { launch, CRED, authedContext, openSession, runCmd, waitTermText, panel } from './lib/browser.mjs';

// rig 配置对齐本目录 README/lib/server.mjs 既有环境变量机制（09-review WR-04：
// 消除硬编码内网 IP——换机器/换人可运行）：WESH_UAT_SSH（必填）/WESH_UAT_SSH_PORT/
// WESH_UAT_TARGET_HOST；凭据单一事实源 = lib/browser.mjs 的 CRED（WESH_UAT_CRED
// 覆盖机制），setup 时经 ssh stdin 递交 ctl——两侧同源，不再「pw 可覆盖/ctl
// 硬编码」分叉导致 T1/T2 静默 401 变红。
const SSH = process.env.WESH_UAT_SSH || '';
const SSH_PORT = process.env.WESH_UAT_SSH_PORT || '22';
if (!SSH) throw new Error('WESH_UAT_SSH 未设置（形态 user@host；端口经 WESH_UAT_SSH_PORT，默认 22）。见 web/uat/pw/README.md');
const TARGET_HOST = process.env.WESH_UAT_TARGET_HOST || (SSH.includes('@') ? SSH.split('@')[1] : SSH);
const BASE = `http://${TARGET_HOST}:10014`; // 浏览器直连 Linux 侧 LAN（端口与 a2 的 10013 错开；安全组放通为前提）
const IDLE_MS = 65_000; // >60s 空闲窗（预期：Caddy hijack 后无默认 WS idle 超时——不断连）
const CTL_NAME = 'phase09-caddy-ctl.sh';
const CTL_LOCAL = fileURLToPath(new URL(`./${CTL_NAME}`, import.meta.url)).replaceAll('\\', '/');
const AUTH_HEADER = 'Basic ' + Buffer.from(CRED).toString('base64');

const results = [];
const ssh = (cmd, opts = {}) => execSync(`ssh -o BatchMode=yes -p ${SSH_PORT} ${SSH} ${JSON.stringify(cmd)}`, { encoding: 'utf8', ...opts }).trim();
const ctl = (args) => ssh(`bash /tmp/wesh-uat/${CTL_NAME} ${args}`);

async function setup() {
  execSync(`scp -o BatchMode=yes -P ${SSH_PORT} "${CTL_LOCAL}" ${SSH}:/tmp/wesh-uat/${CTL_NAME}`, { stdio: 'pipe' });
  // 凭据经 stdin 单程递交（不进远端 argv/命令行——ps 不可见；README「凭据勿走
  // ps 可见面」指引同向）；ctl setup 读首行，空读回落一次性默认（ctl 手跑兼容）。
  const out = ssh(`bash /tmp/wesh-uat/${CTL_NAME} setup`, { input: `${CRED}\n` }); // Linux 侧：caddy 二进制幂等部署 + 一次性 Caddy(LAN) + loopback wesh
  if (!out.includes('CADDY_UP')) throw new Error(`Linux 侧 setup 失败: ${out}`);
}
async function teardown() {
  try { ctl('teardown'); } catch {}
}

const t1 = new Check('CADDY-T1', '页面经 Caddy 反代：无凭据 → 401，带凭据 → 200（challenge 穿透）');
const t2 = new Check('CADDY-T2', '浏览器打开页面 → 终端就绪 → echo 全链（WS 经 Caddy 升级）');
const t3 = new Check('CADDY-T3', `idle ${IDLE_MS / 1000}s 连接不断（无默认 idle 超时）+ 无断连状态面板`);
const t4 = new Check('CADDY-T4', 'teardown 后 10014/17682 端口归零');

mkdirSync('screenshots', { recursive: true });
let browser;
try {
  await setup();
  browser = await launch();

  // T1: request 层 401→200（裸 context 构造——authedContext 的预置头会掩掉 401 负面对照；
  // 401 后 sleep 1.2s 消解 wesh per-IP 节流 fail#1 窗口，05-09/07-07 pacing 纪律）
  const bare = await browser.newContext();
  const r401 = await bare.request.get(`${BASE}/`, { maxRedirects: 0 });
  t1.ok(r401.status() === 401, '无凭据 GET / → 401', `got=${r401.status()}`);
  await sleep(1200);
  const r200 = await bare.request.get(`${BASE}/`, { maxRedirects: 0, headers: { Authorization: AUTH_HEADER } });
  const body200 = await r200.text();
  t1.ok(r200.status() === 200 && body200.includes('<html'), '带凭据 GET / → 200 终端页',
    `got=${r200.status()} html=${body200.includes('<html')}`);
  await bare.close();

  // T2: 浏览器打开页面 → 终端就绪（openSession 含 200 + xterm-rows + 提示符）→ echo 全链
  const ctx = await authedContext(browser);
  const page = await ctx.newPage();
  await openSession(page, `${BASE}/`);
  t2.ok(true, '页面加载 + 终端就绪（xterm-rows + shell 提示符）');
  const MK1 = 'CDYWS_' + Math.random().toString(36).slice(2, 8).toUpperCase();
  await runCmd(page, `echo ${MK1}:$((6*7))`);
  await waitTermText(page, new RegExp(`${MK1}:42`), 10000);
  t2.ok(true, 'echo 标记回读（WS 经 Caddy 升级 + 输入输出全链）', `${MK1}:42`);
  await page.screenshot({ path: 'screenshots/caddy-home.png' });

  // T3: 空闲 >60s 无操作 → 连接不断、面板无异常、终端仍可用
  await sleep(IDLE_MS);
  const p = await panel(page);
  t3.ok(p.hidden, '空闲期间无断连状态面板', `hidden=${p.hidden}`);
  const MK2 = 'CDYIDLE_' + Math.random().toString(36).slice(2, 6).toUpperCase();
  await runCmd(page, `echo ${MK2}:ok`);
  await waitTermText(page, new RegExp(`${MK2}:ok`), 10000);
  t3.ok(true, `空闲 ${IDLE_MS / 1000}s 后 echo 仍可达（Caddy 无默认 WS idle 超时）`, MK2);
  await page.screenshot({ path: 'screenshots/caddy-idle.png' });
  await page.close();
  await ctx.close();

  // T4: teardown → 端口归零（probe 经 ssh 回读 Linux 侧 ss -ltn 计数）
  ctl('teardown');
  const probe = ctl('probe');
  const mC = /listen_caddy=(\d+)/.exec(probe);
  const mW = /listen_wesh=(\d+)/.exec(probe);
  t4.ok(mC?.[1] === '0' && mW?.[1] === '0', 'teardown 后端口归零（ss -ltn 计数）',
    `caddy=${mC?.[1] ?? '?'} wesh=${mW?.[1] ?? '?'}`);
} catch (e) {
  console.log(`  FAIL  场景异常: ${e.message}`);
  results.push(false);
} finally {
  if (browser) await browser.close().catch(() => {});
  await teardown();
}

for (const t of [t1, t2, t3, t4]) {
  const s = t.summary();
  if (s.total > 0) results.push(s.pass);
}
const failed = results.filter((r) => !r).length;
console.log(`\n结果: ${results.length - failed}/${results.length} 项通过`);
process.exit(failed ? 1 : 0);
