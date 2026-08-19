// sanitizeTitle 回归锁（node --test 直跑 .ts——Node 24 内建 type stripping 零新依赖，RESEARCH §A3；
// 相对导入必须带 .ts 扩展名）。本文件只经 node --test 执行，不参与 tsc（tsconfig exclude）。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sanitizeTitle } from './title.ts';

test('纯 ASCII 原样返回', () => {
  assert.equal(sanitizeTitle('user@host: ~/projects/wesh'), 'user@host: ~/projects/wesh');
});

test('C0 控制字符（U+0000-001F，含 \\r\\n\\t）被剥离', () => {
  assert.equal(sanitizeTitle('a\rb\nc\td f'), 'abcd f');
});

test('DEL(U+007F) 与 C1(U+0080-009F) 被剥离', () => {
  assert.equal(sanitizeTitle('vim \u007ffile.txt'), 'vim file.txt');
  assert.equal(sanitizeTitle('x\u0085y\u009fz'), 'xyz');
});

test('Cf 格式控制字符（零宽/bidi 覆盖/isolate/BOM）被剥离', () => {
  assert.equal(sanitizeTitle('\u202emoc.live'), 'moc.live'); // RLO 视觉反转钓鱼
  assert.equal(sanitizeTitle('a\u200bb\u200fc'), 'abc'); // ZWSP / RLM
  assert.equal(sanitizeTitle('x\u2066y\u2069z'), 'xyz'); // bidi isolate
  assert.equal(sanitizeTitle('\ufeffhost'), 'host'); // BOM/ZWNBSP
});

test('超出 128 code point 截断且长度恰 128', () => {
  const out = sanitizeTitle('a'.repeat(200));
  assert.equal(out.length, 128);
  assert.equal(out, 'a'.repeat(128));
});

test('emoji（surrogate pair）截断不拆对——结果不含孤立代理', () => {
  // 127 个 ASCII + 5 个 emoji：第 128 个 code point 恰为 emoji；
  // 按 code point 截断不拆 surrogate pair（Array.from 语义）
  const raw = 'a'.repeat(127) + '🙂'.repeat(5);
  const out = sanitizeTitle(raw);
  assert.equal(Array.from(out).length, 128);
  assert.equal(out.length, 129); // 127 ASCII + 1 emoji（2 个 UTF-16 单元）
  const last = out.charCodeAt(out.length - 1);
  assert.ok(!(last >= 0xd800 && last <= 0xdbff), '末尾不得为孤立 high surrogate');
});

test('全控制字符输入回退 wesh', () => {
  assert.equal(sanitizeTitle('\u0001\u001f\u007f\u0085\u009f'), 'wesh');
});

test('空串回退 wesh（不清空标签页标题）', () => {
  assert.equal(sanitizeTitle(''), 'wesh');
});
