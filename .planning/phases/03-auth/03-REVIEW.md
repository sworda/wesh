---
phase: 03-auth
reviewed: 2026-08-17T12:20:23Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - internal/proto/proto.go
  - internal/proto/proto_test.go
  - internal/pty/spawn_test.go
  - internal/server/auth.go
  - internal/server/auth_e2e_test.go
  - internal/server/auth_test.go
  - internal/server/e2e_test.go
  - internal/server/headers.go
  - internal/server/origin.go
  - internal/server/origin_test.go
  - internal/server/server.go
  - internal/server/throttle.go
  - internal/server/throttle_test.go
  - internal/server/tickets.go
  - internal/server/tickets_test.go
  - internal/server/tls.go
  - internal/server/tls_test.go
  - README.md
  - web/src/main.ts
  - web/uat/phase02.mjs
  - web/uat/phase03.mjs
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 3: Code Review Report

**Reviewed:** 2026-08-17T12:20:23Z
**Depth:** standard
**Files Reviewed:** 23（`web/dist/index.html` 为 0.4MB 前端打包产物，按 dist/ 生成文件规则排除，未做源码级审查）
**Status:** issues_found

## Summary

审查了 Phase 3 认证与传输安全的全部落地代码：CLI 解析与启动校验矩阵（main.go）、常数时间凭据比较（auth.go）、一次性 ticket（tickets.go）、per-IP 指数退避节流（throttle.go）、Origin 白名单（origin.go）、TLS 下限（tls.go）、安全头（headers.go）、WS 握手与守卫链（server.go）、协议常量（proto.go）、前端认证流程（web/src/main.ts）及配套测试/UAT。

核心安全路径经对抗性走查**未发现 BLOCKER**：

- `matchCredential` 为修正后的 `|=` 非短路形态（planner erratum 已正确落地），SHA-256 等长化 + `subtle.ConstantTimeCompare`，长度侧信道与组序号时序泄露均已封堵；
- ticket 签发用 `crypto/rand` 16B 独立 secret、查即删单次使用、过期/非法/重放同口径（go.mod 为 go 1.26.3，`crypto/rand.Read` 自 Go 1.24 起契约保证不返回错误，`_, _ =` 忽略安全）；
- 节流器级数/cap/惰性过期/只读窗口不变量与测试断言逐点吻合；
- 启动校验矩阵为纯函数且先于 listen/spawn，拒绝路径零资源占用，警告文案不含凭据值（有测试锁定）；
- **重点核实**了 `AcceptOptions.OriginPatterns` 的库层语义（coder/websocket v1.8.15 accept.go:243-255）：带 `://` 的 pattern 匹配 `scheme://host`——与本实现喂入的规范化串（`scheme://host[:port]`）一致，注释所称"库内二次校验"成立，非缺陷；
- 半开计数器 acquire/release 恰好一次不变量经 sync.Once + defer 覆盖全部 return 路径，无泄漏无双重释放；
- 日志红线（凭据/ticket/Authorization 永不出现在日志）在实现与测试中均成立。

发现的问题集中在 IPv6 支持缺口（2 项）与一项文档未覆盖的 CSWSH 残余风险。

## Warnings

### WR-01: `--bind` 无法使用 IPv6 地址（listen 地址拼接缺方括号）

**File:** `cmd/wesh/main.go:174`
**Issue:** `net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.bind, cfg.port))` 直接以冒号拼接 bind 与端口。`--bind ::1` / `--bind ::` / `--bind ::ffff:127.0.0.1` 等 IPv6 字面量会拼出 `::1:7681`，`net.Listen` 报 "too many colons in address" 退出。注意 `isLoopbackBind("::1")`（main.go:113-119）能正确识别 IPv6 loopback——即校验放行、listen 失败，用户拿到的是误导性错误而非配置拒绝。当前无任何测试/UAT 覆盖 IPv6 bind（phase02/phase03.mjs 恒 `--bind 127.0.0.1`）。
**Fix:**
```go
ln, err := net.Listen("tcp", net.JoinHostPort(cfg.bind, strconv.Itoa(cfg.port)))
```
`net.JoinHostPort` 对 IPv4/主机名输出与现状逐字相同（`0.0.0.0:7681`），仅对 IPv6 加方括号，行为零漂移。建议补一行 `parseArgs`+`isLoopbackBind("::1")` 的表驱动用例。

### WR-02: `--origin` 的 glob 字符拒绝误伤 IPv6 字面量 Origin

**File:** `internal/server/origin.go:33`
**Issue:** `strings.ContainsAny(s, "*?[\\")` 在原始输入上拒绝 `[`（path.Match glob 元字符），使合法 IPv6 Origin 如 `https://[::1]:8443` 永远无法配置进白名单——`--origin https://[::1]` parse 期即报 "glob characters"，属误报。同源 IPv6 访问不受影响（`originAllowed` ② 的 `EqualFold(r.Host, u.Host)` 命中），仅跨源白名单场景被拒。origin_test.go 只锁了畸形 `http://[::1` 拒绝，未锁定合法 IPv6 形态的预期行为（接受或明确拒绝均应有测试钉死）。
**Fix:** 二选一：
1. 接受 IPv6：glob 检查改为只扫 scheme 与 host 之外的部分，或对 `u.Hostname()` 为 IP 字面量的形态跳过 `[` 拒绝（`[` 在 host 字面量位置不构成 glob 风险，因为 OriginPatterns 比较目标是 `scheme://host`，IPv6 的方括号在两侧同时出现且位置固定）；
2. 显式拒绝并文档化：在 `NormalizeOrigin` 注释与 README `--origin` 行注明"IPv6 字面量 Origin 不支持白名单，同源访问不受影响"，并在 origin_test.go 补一条 `https://[::1]:8443` 的拒绝用例把行为钉死。

### WR-03: 同源 Origin 检查存在 DNS rebinding CSWSH 残余（文档未覆盖）

**File:** `internal/server/server.go:300`（⓪ 调用点）、`internal/server/origin.go:70`
**Issue:** 同源判定为 `strings.EqualFold(r.Host, u.Host)`——无 Host 白名单兜底。DNS rebinding 场景：受害者浏览 `attacker.com:7681`，攻击者将 DNS 重绑定到 `127.0.0.1`，浏览器随后对 `http://attacker.com:7681/ws` 的请求直达本机 wesh，且 Host 与 Origin host 相同（同为 attacker.com:7681），同源检查必然放行。影响面分级：
- **认证模式**：WS 侧还需一次性 ticket，攻击者无凭据无法取 ticket → 被 `auth_failed` 闸住，实际不可利用；
- **loopback 裸跑（`--bind 127.0.0.1` 无凭据，README 明示的合法默认用法）**：无 ticket 闸 → 默认 ro 下攻击者可经受害者浏览器**实时观看终端输出**（屏幕内容即机密面）；`--writable` 下升级为**完整交互 shell**。
这是 coder/websocket 库内建语义（accept.go:239 同款 EqualFold）与本实现 ⓪ 的共有属性，属"所选用缓解措施类别的已知残余"，但 03-CONTEXT/RESEARCH 的威胁模型（"CSWSH 只约束浏览器，浏览器必发 Origin"）未覆盖 rebinding——浏览器确实发了 Origin，只是 Host 与 Origin 同被攻击者域名控制。
**Fix:** 短期在 README 安全说明补一句残余风险提示（"loopback 裸跑模式下 DNS rebinding 可绕过 Origin 同源检查，不可信网页浏览环境建议配置凭据"）；中期（可挂 Phase 7 SEC-07 同批）增加 Host 白名单校验：默认仅放行 `localhost`/`127.0.0.0/8`/`::1` 与实际 bind 地址，`Origin` host 与 Host 均不在白名单时 403。

## Info

### IN-01: 双层 Origin 校验在"显式默认端口"输入上语义不完全同构

**File:** `internal/server/server.go:336`、`internal/server/origin.go:38-39`
**Issue:** 注释称库层 OriginPatterns 为"⓪ 已前置同语义检查"，但存在一处不对称：入站 Origin 带显式默认端口（如 `https://portal.example:443`）时，⓪ 的 `originAllowed` 经 `NormalizeOrigin` 剥默认端口后命中白名单放行，而库层 `path.Match` 不剥端口（accept.go:244-248 直接比 `scheme://host`）→ 库层 403 拒绝。真实浏览器按 RFC 6454 序列化永不发默认端口，实际影响约为零；仅非浏览器客户端手构 Origin 且 Host≠Origin host 时可观测。origin_test.go:70 的"带默认端口规范化命中"用例只测到 `originAllowed` 层，未覆盖 Accept 全链路。
**Fix:** 将 server.go:331-333 的注释从"同语义"修订为"库层不剥默认端口，差异仅在非浏览器显式默认端口 Origin 时可观测，方向保守（多拒不少放）"；或把 `originList` 每项同时注入带/不带默认端口两形态（不推荐，复杂度换不到收益）。

### IN-02: 服务端不校验 WS 帧 message type，text 帧可按二进制协议处理

**File:** `internal/server/server.go:384`（Hello 首读）、`internal/server/server.go:447`（稳态读循环）
**Issue:** `c.Read(ctx)` 返回的 message type 被丢弃（`_`），仅按 `data[0]` 分派。客户端以 MessageText 发送 `'H'+JSON` 会被当作合法 Hello 受理；协议声明"所有帧为 WebSocket 二进制帧"（README 协议节）但服务端未强制。无安全影响（认证/节流/读上限均在更外层生效），纯协议严格性缺口；前端与全部测试客户端均发二进制，零现实触发面。
**Fix:** 如需钉死协议，首读与读循环各加一行 `if mt != websocket.MessageBinary { 1002 关闭 }`；或在 proto.go 帧格式注释中明示"type 不强制，按载荷首字节分派"。当前形态亦可作为有意的前向兼容选择保留——建议至少注释标注为已裁决项，防后人误当疏漏"修复"引入行为漂移。

---

_Reviewed: 2026-08-17T12:20:23Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
