// Phase 07 UAT A2 自动化：真实 nginx 反代挂载观感（OPS-02）
// 拓扑（phase06-pw 先例，见 web/uat/pw/README.md 双机模型）：Linux 侧专用一次性 nginx
// （/tmp/wesh-uat/a2-nginx，LAN 绑定 + auth_basic 一次性 UAT 凭据）→ loopback wesh
// --base-path /wesh；Windows 侧 Playwright Chromium 直连 LAN IP。ssh -L 隧道方案在
// 本机环境实测不稳定（转发进程随浏览器活动死亡，已弃用）。
// 断言：裸 /wesh 308 → /wesh/ 加载、WS 升级终端可用、idle >60s 不断
// （proxy_read_timeout 3600s > --ping-interval 5s）、无精确块变体裸路径 404。
// 红线：凭据值只作构造材料，永不进 detail/控制台输出（只打状态码/布尔/文案常量）。
import { execSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { Check, sleep } from './lib/check.mjs';
import { launch, CRED, openSession, runCmd, waitTermText, panel } from './lib/browser.mjs';

const SSH = '9.134.229.124';
const BASE = 'http://9.134.229.124:10013'; // LAN 直连（安全组已放通，连通性实证）
const IDLE_MS = 65_000; // >60s 空闲窗（A2 预期：proxy_read_timeout 3600s 不断连）
const CTL_LOCAL = fileURLToPath(new URL('./phase07-a2-ctl.sh', import.meta.url)).replaceAll('\\', '/');
// nginx auth_basic 一次性 UAT 凭据：浏览器走 httpCredentials（Chromium 原生 401 应答，
// WS 握手同栈覆盖——extraHTTPHeaders 不到达 WS 握手，实测教训）；request 层显式头。
const [AUTH_USER, AUTH_PASS] = CRED.split(':');
const AUTH_HEADER = 'Basic ' + Buffer.from(CRED).toString('base64');

const results = [];
const ssh = (cmd) => execSync(`ssh -o BatchMode=yes ${SSH} ${JSON.stringify(cmd)}`, { encoding: 'utf8' }).trim();
const ctl = (args) => ssh(`bash /tmp/wesh-uat/a2-ctl.sh ${args}`);

async function setup() {
  execSync(`scp -o BatchMode=yes "${CTL_LOCAL}" ${SSH}:/tmp/wesh-uat/a2-ctl.sh`, { stdio: 'pipe' });
  const out = ctl('setup'); // Linux 侧：一次性 nginx（auth_basic）+ loopback wesh 实例
  if (!out.includes('NGINX_UP')) throw new Error(`Linux 侧 setup 失败: ${out}`);
}
async function teardown() {
  try { ctl('teardown'); } catch {}
}

const t1 = new Check('A2-T1', '裸 /wesh 308 重定向到 /wesh/（精确块）');
const t2 = new Check('A2-T2', '经裸路径访问：页面加载 + 终端就绪');
const t3 = new Check('A2-T3', 'WS 升级成功且终端可用（echo 全链）');
const t4 = new Check('A2-T4', `idle ${IDLE_MS / 1000}s 连接不断（ping 5s < proxy_read_timeout 3600s）`);
const t5 = new Check('A2-T5', '无精确块变体：裸 /wesh → 404（精确块必要性复核）');

mkdirSync('screenshots', { recursive: true });
let browser;
try {
  await setup();
  browser = await launch();
  const ctx = await browser.newContext({ httpCredentials: { username: AUTH_USER, password: AUTH_PASS } });

  // T1: 裸 /wesh（无尾斜杠）→ 308 且 Location 指 /wesh/（request 层断言，不经浏览器跳转）
  const respBare = await ctx.request.get(`${BASE}/wesh`, { maxRedirects: 0, headers: { Authorization: AUTH_HEADER } });
  const loc = respBare.headers()['location'] ?? '';
  t1.ok(respBare.status() === 308, 'status 308', `got=${respBare.status()}`);
  t1.ok(loc.endsWith('/wesh/'), 'Location 尾斜杠形态', `形态=${loc.endsWith('/wesh/')}`);

  // T2: 浏览器经裸路径进入 → 跟随 308 → 终端就绪（openSession 含 200 + xterm-rows + 提示符）
  const page = await ctx.newPage();
  await openSession(page, `${BASE}/wesh`);
  t2.ok(page.url().includes('/wesh/'), '落定 URL 含 /wesh/', `含=${page.url().includes('/wesh/')}`);
  t2.ok(true, '终端就绪（xterm-rows + shell 提示符）');
  await page.screenshot({ path: 'screenshots/a2-home.png' });

  // T3: echo 全链（WS 升级成功的用户可观测等价物）
  const MK1 = 'A2WS_' + Math.random().toString(36).slice(2, 8).toUpperCase();
  await runCmd(page, `echo ${MK1}:$((6*7))`);
  await waitTermText(page, new RegExp(`${MK1}:42`), 10000);
  t3.ok(true, 'echo 标记回读（WS 升级 + 输入输出全链）', `${MK1}:42`);

  // T4: 空闲 >60s 无输入 → 连接不断、面板无异常、终端仍可用
  await sleep(IDLE_MS);
  const p = await panel(page);
  t4.ok(p.hidden, '空闲期间无断连状态面板', `hidden=${p.hidden}`);
  const MK2 = 'A2IDLE_' + Math.random().toString(36).slice(2, 8).toUpperCase();
  await runCmd(page, `echo ${MK2}:ok`);
  await waitTermText(page, new RegExp(`${MK2}:ok`), 10000);
  t4.ok(true, `空闲 ${IDLE_MS / 1000}s 后终端仍可用`, MK2);
  await page.screenshot({ path: 'screenshots/a2-idle.png' });
  await page.close();

  // T5: 无精确块变体 — 裸 /wesh 应 404（C1 语义在线上复证）；/wesh/ 仍 200
  const v = ctl('variant noexact');
  if (!v.includes('RELOADED_noexact')) throw new Error(`变体切换失败: ${v}`);
  const resp404 = await ctx.request.get(`${BASE}/wesh`, { maxRedirects: 0, headers: { Authorization: AUTH_HEADER } });
  t5.ok(resp404.status() === 404, '无精确块裸 /wesh → 404', `got=${resp404.status()}`);
  const resp200 = await ctx.request.get(`${BASE}/wesh/`, { maxRedirects: 0, headers: { Authorization: AUTH_HEADER } });
  t5.ok(resp200.status() === 200, '前缀块 /wesh/ 仍 200', `got=${resp200.status()}`);
  ctl('variant exact'); // 还原精确块（teardown 会整体清理，双保险）

  await ctx.close();
} catch (e) {
  console.log(`  FAIL  场景异常: ${e.message}`);
  results.push(false);
} finally {
  if (browser) await browser.close().catch(() => {});
  await teardown();
}

for (const t of [t1, t2, t3, t4, t5]) {
  const s = t.summary();
  if (s.total > 0) results.push(s.pass);
}
const failed = results.filter((r) => !r).length;
console.log(`\n结果: ${results.length - failed}/${results.length} 项通过`);
process.exit(failed ? 1 : 0);
