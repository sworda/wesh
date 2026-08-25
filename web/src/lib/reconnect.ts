// CORE-05 重连退避纯函数（node --test 可测——零 DOM 依赖，prefs.ts 同款形态）。
// D-02 参数族：1s×2 封顶 30s——internal/server/throttle.go:12-13 同族参数族
//（defaultThrottleBase = 1s / defaultThrottleCap = 30s，P3 D-08 形态延伸）。

// 退避毫秒数：1s×2 封顶 30s 无限重试——无尝试次数上限，个人运维「标签页放着，
// 回来已接回」主场景，30s 一次重试流量可忽略（D-02）；重连成功（WELCOME 到达）退避清零
export function backoffMs(attempt: number): number {
  return Math.min(1000 * 2 ** attempt, 30000);
}

// 触发谓词显式判定（D-01）——1002 协议错误等带码关闭留 default 桶手动面板，
// 1013 被踢维持手动刷新（P5 D-10）；1006 为浏览器本地合成码永不出现于线上（RFC6455 §7.4）
export function shouldReconnect(code: number): boolean {
  return code === 1006;
}
