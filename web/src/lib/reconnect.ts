// CORE-05 重连退避纯函数（node --test 可测——零 DOM 依赖，prefs.ts 同款形态）。
// D-02 参数族：1s×2 封顶 30s——internal/server/throttle.go:12-13 同族参数族
//（defaultThrottleBase = 1s / defaultThrottleCap = 30s，P3 D-08 形态延伸）。

// 退避毫秒数：1s×2 封顶 30s 无限重试——无尝试次数上限，个人运维「标签页放着，
// 回来已接回」主场景，30s 一次重试流量可忽略（D-02）；重连成功（WELCOME 到达）退避清零
export function backoffMs(attempt: number): number {
  return Math.min(1000 * 2 ** attempt, 30000);
}

// 触发谓词显式判定（D-01），2026-09-13 扩码集：
//   - 1006 浏览器本地合成网络异常（永不出现于线上，RFC6455 §7.4）；
//   - 1001 服务端优雅重启/关停（D-23 反转：会话已持久化，重启后 cookie 仍有效，
//     自动重连把 systemd restart 变成用户无感——重连循环打重启中服务 1s→30s
//     退避可接受）；
//   - 1011 容量满 / spawn 节流等瞬态服务端错误（退避重试而非终态面板）。
// 1002 协议错误 / 1008 策略违反 / 1009 超限维持终态面板（default 桶）。
export function shouldReconnect(code: number): boolean {
  return code === 1006 || code === 1001 || code === 1011;
}

// 1013 慢消费者被踢：仅页面前台可见时重连——后台标签被浏览器节流正是成为慢
// 消费者的主因，隐藏时重连只会再被踢（D-10 原意保持）；前台可见则值得自愈。
export function reconnectOnKick(visible: boolean): boolean {
  return visible;
}
