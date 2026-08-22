// G-05-1 终端核心层断言（@xterm/headless）：异尺寸双端会话尺寸约束渲染的等价锁 + 负对照。
//
// 血缘：本脚本把 2026-08-22 G-05-1 根因诊断期的临时探针 probe10.mjs 机制转正为门禁断言
//（探针为一次性诊断用具，从未入库——05-UAT.md Gaps G-05-1 root_cause 节登记的
//「40 列 PTY 字节流喂 120 列 xterm，长命令回显换行点分叉」实证即出自该探针；
// 机制描述见 05-12-PLAN Task 2，此处按 plan 重建为零依赖传统之外的 @xterm/headless 形态）。
//
// 机理（G-05-1 叠写根因与修复的可断言等价物）：
//   readline/echo 等相对寻址流按 PTY 宽度（= 会话尺寸 40 列）生成环绕点；
//   - D6H-1 等价锁：B（宽端旁观 120x40）收到的字节流喂「120x40 建起再 resize(40,10)」的
//     headless（前端约束渲染的精确等价物——xterm 按 fit 尺寸创建、WELCOME 到达后
//     term.resize 到会话尺寸，见 web/src/main.ts refit()），与 A（窄端 owner 40x10 原生）
//     流喂另一 40x10 headless 的 buffer 逐行全等——「同 cols 渲染同字节流 = 逐屏严格一致」
//     在终端核心层锁定（修复后行为：宽端约束渲染 ≡ 窄端原生渲染）。
//   - D6H-2 负对照：同一 B 流喂 120x40 headless（修复前的无约束形态）→ 与 40 列渲染
//     逐行**不全等**（换行点分叉 = G-05-1 叠写机理复现；phase04-t1-width.mjs U6 对照组
//     先例——证明 D6H-1 的断言有区分度而非恒真）。
//
// 与 phase05.mjs S10（协议面 carriage/推送/升格）、phase05-dom.mjs D6（DOM 约束渲染）
// 构成 G-05-1 三层自动化回归锁。headless 与浏览器走同一 xterm buffer 代码路径
//（根 CODEBUDDY.md 四层测试策略第 2 层；allowProposedApi 纪律同款）。
//
// 运行：node web/uat/phase05-dims.mjs [wesh 二进制路径]（默认 /tmp/wesh-uat/wesh）
import { spawn } from 'node:child_process';
import pkg from '@xterm/headless';
const { Terminal } = pkg;

const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';

// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, HELLO = 0x48, WELCOME = 0x57;
const SUBPROTOCOL = 'wesh.v1';

const enc = new TextEncoder();
const dec = new TextDecoder();
const concat = (...parts) => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) { out.set(p, off); off += p.length; }
  return out;
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// 结果汇总/退出码形态照 phase04-t1-width.mjs（check(name, actual, expected) + process.exit）
let failed = 0;
const check = (name, actual, expected) => {
  const ok = actual === expected;
  if (!ok) failed++;
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${name} — expected=${expected} actual=${actual}`);
};

// startWesh/dialHello/outputBytes 照 phase05.mjs 既有夹具逐字形态（端口解析正则同款）。
// 本脚本不走分享链接（无认证模式裸 Hello），无 token 面——红线天然不触。
function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    let stderr = '';
    let stdoutBuf = '';
    let settling = false;
    const to = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`wesh 启动超时: ${args.join(' ')}; stderr=${stderr}`)); }, 8000);
    child.stderr.on('data', (d) => { stderr += d; });
    child.stdout.on('data', (d) => {
      stdoutBuf += d.toString();
      if (settling) return;
      const m = /listening on (https?):\/\/[^\s]+:(\d+)/.exec(stdoutBuf);
      if (m && stdoutBuf.includes('share read-only:')) {
        settling = true;
        clearTimeout(to);
        setTimeout(() => resolve({ port: Number(m[2]), kill: () => child.kill('SIGKILL') }), 50);
      }
    });
    child.on('error', (e) => { clearTimeout(to); reject(e); });
  });
}

function dialHello(port, { cols = 80, rows = 24 } = {}) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, [SUBPROTOCOL]);
    ws.binaryType = 'arraybuffer';
    const frames = [];
    ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
    ws.onopen = () => ws.send(concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols, rows }))));
    ws.onerror = () => reject(new Error('WS 连接失败'));
    const watchdog = setTimeout(() => { clearInterval(poll); reject(new Error('握手总超时：10s 未收到 Welcome')); }, 10000);
    const poll = setInterval(() => {
      if (frames.some((f) => f[0] === WELCOME)) { clearInterval(poll); clearTimeout(watchdog); resolve({ ws, frames }); }
    }, 10);
    ws.onclose = (ev) => { clearInterval(poll); clearTimeout(watchdog); reject(new Error(`握手被关闭 code=${ev.code}`)); };
  });
}

const outputText = (frames, fromIdx = 0) =>
  frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => dec.decode(f.subarray(1))).join('');
const outputBytes = (frames, fromIdx = 0) =>
  Buffer.concat(frames.slice(fromIdx).filter((f) => f[0] === OUTPUT).map((f) => Buffer.from(f.subarray(1))));

const write = (term, data) => new Promise((r) => term.write(data, r));

// buffer 快照：全部行（含 scrollback）translateToString(true)（trimRight）后去尾部空行 join——
// 折行点分叉在快照层可见（40 列下 99 字符行 = 40+40+19 三行；120 列下 = 单行），
// 且不受终端 rows 高度差异影响（尾部空行已剥离）
function snapshot(term) {
  const buf = term.buffer.active;
  const lines = [];
  for (let i = 0; i < buf.length; i++) lines.push(buf.getLine(i).translateToString(true));
  while (lines.length > 0 && lines[lines.length - 1] === '') lines.pop();
  return lines.join('\n');
}

const inst = await startWesh(['--writable', '--', 'bash', '--norc', '--noprofile']);
try {
  // A = 窄端 owner（40x10，会话尺寸钉 40x10）；B = 宽端旁观（120x40，D-07 降级 ro）
  const a = await dialHello(inst.port, { cols: 40, rows: 10 });
  const b = await dialHello(inst.port, { cols: 120, rows: 40 });

  // prompt 落定（A 端首输出非空白）后取各自 base 切片
  const t0 = Date.now();
  while (Date.now() - t0 < 8000 && outputText(a.frames).trim().length === 0) await sleep(50);
  const baseA = a.frames.length;
  const baseB = b.frames.length;

  // A 注入超 40 列长命令（echo 标记串 + 长尾巴）：命令回显与输出均按 PTY 40 列环绕；
  // DONE 标记为双端齐读信号
  const MARK = 'UAT_DIMS_' + 'X'.repeat(90); // 99 字符 > 40 列两倍，换行点分叉材料
  a.ws.send(concat(new Uint8Array([INPUT]), enc.encode(`echo ${MARK}; echo UAT_DIMS_DONE\n`)));
  const t1 = Date.now();
  let doneA = false, doneB = false;
  while (Date.now() - t1 < 10000 && !(doneA && doneB)) {
    if (!doneA) doneA = outputText(a.frames, baseA).includes('UAT_DIMS_DONE');
    if (!doneB) doneB = outputText(b.frames, baseB).includes('UAT_DIMS_DONE');
    if (!doneA || !doneB) await sleep(50);
  }
  await sleep(500); // 齐读后等尾随 prompt 帧双端落定（S1b 同款形态）
  const streamA = outputBytes(a.frames, baseA);
  const streamB = outputBytes(b.frames, baseB);
  check('D6H-0 前置：双端字节流收齐（marker 齐读且非空）',
    doneA && doneB && streamA.length > 0 && streamB.length > 0, true);

  // D6H-1 等价锁：窄端原生（40x10 建起）vs 宽端约束渲染等价物（120x40 建起 → resize(40,10)
  // —— 前端 refit() 约束路径的精确形态），同一字节流逐屏严格一致
  const termNarrowA = new Terminal({ cols: 40, rows: 10, allowProposedApi: true });
  await write(termNarrowA, streamA);
  const termConstrainedB = new Terminal({ cols: 120, rows: 40, allowProposedApi: true });
  termConstrainedB.resize(40, 10); // WELCOME 携会话尺寸到达后的 refit 约束（此时 buffer 空）
  await write(termConstrainedB, streamB);
  const snapNarrowA = snapshot(termNarrowA);
  const snapConstrainedB = snapshot(termConstrainedB);
  check('D6H-1 等价锁：同 40 列渲染同一字节流逐屏严格一致（约束渲染 ≡ 窄端原生）',
    snapConstrainedB === snapNarrowA, true);

  // D6H-2 负对照：同一 B 流喂 120 列（修复前无约束形态）→ 换行点分叉，与 40 列渲染不全等
  const termWideB = new Terminal({ cols: 120, rows: 40, allowProposedApi: true });
  await write(termWideB, streamB);
  const snapWideB = snapshot(termWideB);
  check('D6H-2 负对照：同字节流喂 120 列换行点分叉（与 40 列渲染不全等，G-05-1 叠写机理复现）',
    snapWideB === snapConstrainedB, false);

  a.ws.close(); b.ws.close();
} finally {
  inst.kill();
}

console.log(failed === 0 ? '\nDIMS PASS（D6H-1 等价锁 + D6H-2 负对照）' : `\nDIMS FAIL（${failed} 项）`);
process.exit(failed === 0 ? 0 : 1);
