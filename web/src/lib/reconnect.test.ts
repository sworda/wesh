// reconnect 纯函数回归锁（node --test 直跑 .ts——Node 24 内建 type stripping 零新依赖，prefs.test.ts 同款；
// 相对导入必须带 .ts 扩展名）。本文件只经 node --test 执行，不参与 tsc（tsconfig exclude）。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { backoffMs, shouldReconnect } from './reconnect.ts';

test('backoffMs: 1s×2 封顶 30s 退避序列（D-02，attempt 0 起点）', () => {
  const seq = [0, 1, 2, 3, 4, 5, 6].map((a) => backoffMs(a));
  assert.deepEqual(seq, [1000, 2000, 4000, 8000, 16000, 30000, 30000]); // attempt 5 起封顶截断
});

test('backoffMs: 深尝试仍封顶 30s（无限重试无次数上限，D-02）', () => {
  assert.equal(backoffMs(10), 30000);
});

test('shouldReconnect: 仅 1006 触发（D-01 显式谓词——浏览器本地合成码，RFC6455 §7.4）', () => {
  assert.equal(shouldReconnect(1006), true);
  // 1000 正常终结 / 1002 协议错误（留 default 桶手动面板）/ 1008 / 1009 / 1011 / 1013 被踢维持手动刷新
  for (const code of [1000, 1002, 1008, 1009, 1011, 1013]) {
    assert.equal(shouldReconnect(code), false);
  }
});
