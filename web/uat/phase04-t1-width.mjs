// T1 CJK/emoji 宽度自动化断言（@xterm/headless + Unicode11Addon，等价于
// web/src/main.ts:113-114 的 loadAddon→activeVersion='11' 硬顺序）。
// 原理：宽度测量是终端核心 buffer 逻辑，headless 与浏览器走同一代码路径；
// 渲染像素层（字形是否好看）不可测，但"占两格/光标位置"可精确断言。
import pkg from '@xterm/headless';
import pkgU from '@xterm/addon-unicode11';
const { Terminal } = pkg;
const { Unicode11Addon } = pkgU;

let failed = 0;
const check = (name, actual, expected) => {
  const ok = actual === expected;
  if (!ok) failed++;
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${name} — expected=${expected} actual=${actual}`);
};

const write = (term, s) =>
  new Promise((r) => term.write(s, r));

const cursorX = (term) => term.buffer.active.cursorX;

// ── 主测组：Unicode 11 激活（生产配置）──
const t11 = new Terminal({ cols: 80, rows: 24, allowProposedApi: true });
t11.loadAddon(new Unicode11Addon());
t11.unicode.activeVersion = '11';

// 4 个 CJK + 2 个 emoji = 4*2 + 2*2 = 12 格
await write(t11, '中文测试🙂🎉');
check('U11: CJKx4+emoji x2 光标格数', cursorX(t11), 12);

// emoji 后跟 ASCII：emoji 2 格 + X 1 格 = 3（不叠字不多占格）
const t11b = new Terminal({ cols: 80, rows: 24, allowProposedApi: true });
t11b.loadAddon(new Unicode11Addon());
t11b.unicode.activeVersion = '11';
await write(t11b, '🙂X');
check('U11: emoji+ASCII 光标格数', cursorX(t11b), 3);

// CJK 与 ASCII 混排对齐：'中a文b' = 2+1+2+1 = 6
const t11c = new Terminal({ cols: 80, rows: 24, allowProposedApi: true });
t11c.loadAddon(new Unicode11Addon());
t11c.unicode.activeVersion = '11';
await write(t11c, '中a文b');
check('U11: CJK/ASCII 混排光标格数', cursorX(t11c), 6);

// buffer 内容完整性：宽字符占位符结构（宽字符后跟随空 cell）
const line = t11.buffer.active.getLine(0);
const cells = [];
for (let i = 0; i < 12; i++) {
  const c = line.getCell(i);
  cells.push(c.getChars() || '·');
}
check('U11: buffer 行内容', cells.join(''), [...'中文测试🙂🎉'].flatMap((ch) => [ch, '·']).join(''));

// ── 对照组：Unicode 6（未激活 addon 的默认行为）──
// 🙂 U+1F642 在 Unicode 6 表中宽度为 1，证明断言确实区分版本
const t6 = new Terminal({ cols: 80, rows: 24, allowProposedApi: true });
await write(t6, '🙂');
check('U6 对照: emoji 光标格数（证明区分度）', cursorX(t6), 1);

console.log(failed === 0 ? '\nT1 PASS（U11 主测 4/4 + U6 对照 1/1）' : `\nT1 FAIL（${failed} 项）`);
process.exit(failed === 0 ? 0 : 1);
