// prefs 纯函数回归锁（node --test 直跑 .ts——Node 24 内建 type stripping 零新依赖，RESEARCH §A3；
// 相对导入必须带 .ts 扩展名）。本文件只经 node --test 执行，不参与 tsc（tsconfig exclude）。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseQueryPrefs, splitPrefs, mergeTheme } from './prefs.ts';

test('parseQueryPrefs: 合法白名单键解析并记入 keys', () => {
  const r = parseQueryPrefs('?fontSize=16&cursorBlink=false');
  assert.equal(r.prefs.fontSize, 16);
  assert.equal(r.prefs.cursorBlink, false);
  assert.ok(r.keys.has('fontSize'));
  assert.ok(r.keys.has('cursorBlink'));
  assert.deepEqual(r.invalid, []);
});

test('parseQueryPrefs: 裸词非法 JSON 跳过并记入 invalid（UI-SPEC 示例行 fontFamily=Menlo）', () => {
  const r = parseQueryPrefs('?fontFamily=Menlo');
  assert.equal(r.prefs.fontFamily, undefined);
  assert.ok(!r.keys.has('fontFamily'));
  assert.deepEqual(r.invalid, ['fontFamily']);
});

test('parseQueryPrefs: 白名单外未知键跳过', () => {
  const r = parseQueryPrefs('?foo=1');
  assert.deepEqual(r.prefs, {});
  assert.equal(r.keys.size, 0);
  assert.deepEqual(r.invalid, []);
});

test('parseQueryPrefs: osc52 结构性排除——query 永不可开启 OSC52（D-12 专项）', () => {
  const r = parseQueryPrefs('?osc52=true');
  assert.deepEqual(r.prefs, {});
  assert.equal(r.keys.size, 0);
});

test('parseQueryPrefs: theme URL 编码 JSON 对象解析为对象', () => {
  const r = parseQueryPrefs('?theme=' + encodeURIComponent('{"background":"#101020"}'));
  assert.deepEqual(r.prefs.theme, { background: '#101020' });
  assert.ok(r.keys.has('theme'));
});

test('splitPrefs: fontSize 分派 xterm、resizeOverlay 分派 behavior、osc52 不进任一组', () => {
  const parts = splitPrefs({ fontSize: 16, resizeOverlay: false, osc52: true });
  assert.deepEqual(parts.xterm, { fontSize: 16 });
  assert.deepEqual(parts.behavior, { resizeOverlay: false });
});

test('mergeTheme: 部分 theme 合并保留 base 未指定键', () => {
  const base = { foreground: '#ffffff', background: '#000000', red: '#cc0000' };
  const merged = mergeTheme(base, { background: '#101020' });
  assert.deepEqual(merged, { foreground: '#ffffff', background: '#101020', red: '#cc0000' });
});

test('mergeTheme: 非对象/数组/null 入参返回 null', () => {
  const base = { foreground: '#ffffff' };
  assert.equal(mergeTheme(base, null), null);
  assert.equal(mergeTheme(base, ['#000']), null);
  assert.equal(mergeTheme(base, '#000'), null);
  assert.equal(mergeTheme(base, 42), null);
});
