// reconnect 纯函数回归锁（node --test 直跑 .ts——Node 24 内建 type stripping 零新依赖，prefs.test.ts 同款；
// 相对导入必须带 .ts 扩展名）。本文件只经 node --test 执行，不参与 tsc（tsconfig exclude）。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { backoffMs, reconnectOnKick, shouldReconnect } from './reconnect.ts';

test('backoffMs: 1s×2 封顶 30s 退避序列（D-02，attempt 0 起点）', () => {
  const seq = [0, 1, 2, 3, 4, 5, 6].map((a) => backoffMs(a));
  assert.deepEqual(seq, [1000, 2000, 4000, 8000, 16000, 30000, 30000]); // attempt 5 起封顶截断
});

test('backoffMs: 深尝试仍封顶 30s（无限重试无次数上限，D-02）', () => {
  assert.equal(backoffMs(10), 30000);
});

test('shouldReconnect: 1006/1001/1011 触发（2026-09-13 扩码集）', () => {
  assert.equal(shouldReconnect(1006), true); // 网络异常（本地合成码，RFC6455 §7.4）
  assert.equal(shouldReconnect(1001), true); // 服务端优雅重启（会话持久化 → 重启无感，D-23 反转）
  assert.equal(shouldReconnect(1011), true); // 容量/spawn 节流瞬态（退避重试）
  // 1000 子进程退出 / 1002 协议错误 / 1008 策略违反 / 1009 超限 / 1013 被踢（另经 reconnectOnKick）维持终态面板
  for (const code of [1000, 1002, 1008, 1009, 1013]) {
    assert.equal(shouldReconnect(code), false);
  }
});

test('reconnectOnKick: 仅前台可见时重连（D-10 原意——后台重连只会再被踢）', () => {
  assert.equal(reconnectOnKick(true), true);
  assert.equal(reconnectOnKick(false), false);
});
