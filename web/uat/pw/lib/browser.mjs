// 浏览器助手：DOM 渲染器强制（.xterm-rows 可断言）、认证上下文、终端读写、状态面板读取。
// --disable-webgl：FE-01 首选 WebGL 渲染器，加载失败自动停留 DOM 渲染器——禁用 WebGL 使
// 文本落入 DOM 可断言（渲染降级路径即产品内建行为，非测试特判）。
import { chromium } from 'playwright';

// 预置 Authorization 避开 401→recordFail→429 节流链（internal/server/auth.go basicAuth 守卫序）
export const CRED = process.env.WESH_UAT_CRED || 'user:pass';

export async function launch() {
  return chromium.launch({ headless: true, args: ['--disable-webgl'] });
}

export function authedContext(browser) {
  return browser.newContext({
    extraHTTPHeaders: {
      Authorization: 'Basic ' + Buffer.from(CRED).toString('base64'),
    },
  });
}

export function termText(page) {
  return page.evaluate(() => document.querySelector('.xterm-rows')?.textContent ?? '');
}

export async function waitForPrompt(page, timeout = 20000) {
  await page.waitForFunction(
    () => /[$#]\s*$/.test((document.querySelector('.xterm-rows')?.textContent ?? '').trimEnd()),
    null,
    { timeout },
  );
}

// 打开会话并等到 shell 提示符就绪
export async function openSession(page, url, { timeout = 25000 } = {}) {
  const resp = await page.goto(url, { waitUntil: 'domcontentloaded', timeout });
  if (resp.status() !== 200) throw new Error(`HTTP ${resp.status()} on ${url}`);
  await page.waitForSelector('.xterm-rows', { timeout });
  await waitForPrompt(page, timeout);
  return page;
}

export async function runCmd(page, cmd) {
  await page.click('.xterm');
  await page.keyboard.type(cmd);
  await page.keyboard.press('Enter');
}

export async function waitTermText(page, re, timeout = 10000) {
  await page.waitForFunction(
    (src) => new RegExp(src).test(document.querySelector('.xterm-rows')?.textContent ?? ''),
    re.source,
    { timeout },
  );
  return termText(page);
}

export function marker(tag = 'MK') {
  return `${tag}_${Math.random().toString(36).slice(2, 8).toUpperCase()}`;
}

// echo PID<nonce>:$$ → 解析输出行的 pid（命令回显行含字面 $$ 不被 \d+ 误配）
export async function getShellPid(page) {
  const nonce = marker('PID');
  await runCmd(page, `echo ${nonce}:$$`);
  const t = await waitTermText(page, new RegExp(`${nonce}:(\\d+)`), 10000);
  const m = t.match(new RegExp(`${nonce}:(\\d+)`));
  return m ? m[1] : null;
}

export async function panel(page) {
  return page.evaluate(() => {
    const el = document.getElementById('status');
    const a = document.querySelector('#status-hint a');
    return {
      hidden: el ? el.hidden : true,
      title: document.getElementById('status-title')?.textContent ?? '',
      body: document.getElementById('status-body')?.textContent ?? '',
      hint: document.getElementById('status-hint')?.textContent ?? '',
      action: a?.textContent ?? null,
    };
  });
}

export async function waitPanel(page, timeout = 12000) {
  const t0 = Date.now();
  await page.waitForFunction(() => document.getElementById('status')?.hidden === false, null, { timeout });
  const p = await panel(page);
  p.latencyMs = Date.now() - t0;
  return p;
}

export async function waitPanelHidden(page, timeout = 8000) {
  const t0 = Date.now();
  await page.waitForFunction(() => document.getElementById('status')?.hidden !== false, null, { timeout });
  return Date.now() - t0;
}

// 合成 online 事件：转发器恢复对 OS 网络栈不可见，真实断网恢复时浏览器由 OS 派发该事件——
// 此处调用同一监听器，语义等价（phase06-dom D8 同形态）。
export async function fireOnline(page) {
  await page.evaluate(() => window.dispatchEvent(new Event('online')));
}

export async function titleOf(page) {
  return page.title();
}

export async function waitTitle(page, re, timeout = 12000) {
  await page.waitForFunction((src) => new RegExp(src).test(document.title), re.source, { timeout });
  return page.title();
}
