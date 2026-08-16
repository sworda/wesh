---
phase: 02-protocol
reviewed: 2026-08-15T12:29:06Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - internal/proto/proto.go
  - internal/proto/proto_test.go
  - internal/pty/io.go
  - internal/pty/io_test.go
  - internal/pty/spawn.go
  - internal/server/e2e_test.go
  - internal/server/handshake_test.go
  - internal/server/keepalive_test.go
  - internal/server/limits_test.go
  - internal/server/server.go
  - README.md
  - web/src/main.ts
findings:
  critical: 1
  warning: 2
  info: 4
  total: 7
status: issues_found
---

# Phase 2: Code Review Report

**Reviewed:** 2026-08-15T12:29:06Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

评审覆盖 wesh Phase 02 协议基线的全部 Go 源码、测试与前端入口。已核实：`go build`/`go vet` 干净，`go test -race -count=1 ./...` 全绿（4 包通过），`tsc --noEmit` 干净；注释中引用的 coder/websocket v1.8.15 与 creack/pty v1.1.24 库行为声明（Close 5s+5s 握手、Ping 需并发 Reader、SetReadLimit +1 余量、selectSubprotocol EqualFold、StartWithSize 注入 Setsid/Setctty）均与模块缓存源码逐条核对一致。

安全守卫链（400→429→409→Accept→assert→4KiB/5s→Hello 状态机→16KiB/pinger）顺序正确，halfOpen acquire/release 恰好一次不变量经 sync.Once+defer 覆盖全部 return 路径，ro/rw 门、关闭路径幂等性（Close/CloseNow/termOnce/releaseOnce）均成立，协议常量前后端对齐且被测试逐字锁定。

主要问题：**发现 1 个 BLOCKER——Attach 读循环同步写 PTY master，在子进程不读 stdin（raw 模式）时阻塞，使 D-11 单次语义退出保证失效并导致 pinger 误杀健康客户端**（已用内核行为探针实证）。另有 2 个 WARNING（onChunk 写无超时使 Drain 时限失效；unknown-frame 关闭原因违反 D-07 机器串纪律且无 stderr 事件）与 4 个 INFO。

## Critical Issues

### CR-01: Attach 读循环阻塞于 `Master.Write`——D-11 退出保证失效 + pinger 误杀健康客户端

**File:** `internal/server/server.go:352`
**Issue:** 稳态读循环在 `case proto.Input:` 分支同步调用 `s.sess.Master.Write(data[1:])`。当子进程把 PTY 设为 raw 模式（任何 TUI：vim/htop/tmux/less 等 wesh 的核心使用场景）且停止读取 stdin 时，n_tty 输入缓冲填满后 master 写 syscall 会**阻塞**（实测：raw 模式 + 不读子进程，写入 20480 字节后 Write 永久阻塞；canonical 模式则静默丢弃不阻塞，故 `sleep` 类命令不触发）。16KiB/帧上限不构成防御——两条 INPUT 消息即可凑满 20KB。

阻塞后产生两个独立故障：

1. **健康客户端被误杀**：Attach goroutine 卡在 Write，不再执行 `c.Read` → 库只在读路径处理 pong（read.go:317-337）→ pinger 的活跃 ping 在 pongTimeout（默认 10s）后必然超时 → `c.CloseNow()` 掐断一条完全健康的连接（浏览器在协议栈层回 pong 也无济于事，服务端读循环没运行就永远看不到 pong）。客户端见 1006，服务端 stderr 留下一条语义失真的 `pong_timeout` 事件。
2. **D-11 失效**：若客户端确实已断开（关标签页/网络分区），`wsDisconnected`/`terminate` 全挂在读循环之后，而读循环卡在 Write——`CloseNow` 只能唤醒阻塞在 `c.Read` 的 goroutine，唤不醒阻塞在 master fd 上的 goroutine。`exitf` 永不触发，wesh 挂死直到子进程恢复读取或退出（`stty raw; sleep infinity` 形态即永久挂死，端口与进程常驻）。

**实测证据**（creack/pty v1.1.24 + `stty raw -echo; sleep 10` 子进程探针）：写入 20480 字节后 `m.Write` 阻塞，3 秒观察窗内无任何返回/错误/短写。

**Fix:** 读写解耦——master 写入移出读循环，例如：

```go
// 有界输入队列 + 独立写 goroutine；满时按背压策略处理（与 Phase 5 的 1013 踢出对齐）
type session struct { inputCh chan []byte /* cap 例如 64 */ ... }

case proto.Input:
    if !s.writable { continue }
    payload := append([]byte(nil), data[1:]...) // 拷贝，读循环缓冲复用
    select {
    case s.inputCh <- payload:
    default:
        // 队列满：丢弃或关连接（Phase 5 决策点），绝不阻塞读循环
    }
```

最低限度缓解（不根治但消除挂死）：master fd 置 `O_NONBLOCK`，`Write` 返回 `ErrWouldBlock` 时走既有 wsDisconnected→terminate 收口。注意仅让 pinger 在 pong_timeout 时补调 `terminate` 不是正解——会把故障 1 的误杀从"断连"升级为"杀整个服务端"（termOnce 保证幂等但语义错误）。

## Warnings

### WR-01: `onChunk` 的 `c.Write(context.Background(), ...)` 无超时——ReadLoop 可永久停滞，`Drain` 时限承诺失效

**File:** `internal/server/server.go:374`（关联 `internal/pty/io.go:15-27`）
**Issue:** S→C 数据泵写 WS 不带任何 deadline。对一个"TCP 层存活、WS 层回 pong、但应用层永不读数据帧"的客户端（自定义客户端最容易构造；浏览器后台标签页在渲染器缓冲打满前的窗口期同形态）：服务端/客户端内核缓冲填满后 `c.Write` 永久阻塞 → ReadLoop 停在 `onChunk` 内 → master 不再 drain → 子进程写满 64KiB PTY 内核缓冲后阻塞、永不退出 → `lifecycle` 卡在 `Wait` → 单次语义服务端整体挂死。

pinger 救不了此路径：Attach 的 `c.Read` 循环独立于 ReadLoop，照常处理 pong，pinger 满意，连接表面健康。`Session.Drain(200ms)` 的"到点无条件 Close(master)"承诺（io.go:62-63 注释）也被击穿——ReadLoop 卡在 WS 写而非 master 读，`Close(master)` 唤不醒它，`readDone` 永不关闭。1013 背压踢出虽属 Phase 5（D-08 占位），但当前形态下 D-12 drain 与 Pitfall-4 时限两项已落地的防线在该场景同步失效，应在本 phase 至少以写超时盒住。

**Fix:** 为 `onChunk` 的写加有界超时（如 `context.WithTimeout(context.Background(), 10*time.Second)`），超时即视为客户端死亡——`c.CloseNow()` 后经既有 reader 路径收口（D-11）；或排队等 Phase 5 背压方案时先在注释/文档中明示该挂死形态。

### WR-02: unknown-frame 关闭原因 `"unknown frame type"` 违反 D-07 机器串纪律，且缺失 D-12② stderr 事件

**File:** `internal/server/server.go:360`
**Issue:** 两处一致性缺陷：

1. D-07 规定"主动关闭的 close reason 带同名机器串（snake_case）"。全部其他违规路径均遵守（`empty_frame`/`frame_before_hello`/`malformed_hello`/`hello_timeout`/`subprotocol_required`/`version_mismatch`），唯独 unknown-frame 路径发送带空格的英文句子 `"unknown frame type"`——与 proto.go:41-42 注释声明的纪律自相矛盾，也让前端按 reason 匹配/抓包归类时遇到独一例外。
2. 该路径是所有握手/协议违规中**唯一不打 `logEvent` stderr 事件**的（对照 logEvent 注释列出的七类事件清单）。攻击面零反馈约束（D-06）只禁止发 Error 帧，close reason 本身已携带信息，补一行 stderr 日志不增加反馈面，反而消除观测盲区。

**Fix:**

```go
default:
    s.logEvent(remote, websocket.StatusProtocolError, "unknown_frame")
    _ = c.Close(websocket.StatusProtocolError, "unknown_frame")
```

## Info

### IN-01: hello-timeout 栅栏竞态——踩点到达的合法 Hello 可能被以 `hello_timeout` 拒绝

**File:** `internal/server/server.go:264-271, 312`
**Issue:** AfterFunc 回调用非阻塞 `select { case <-helloDone: default: ... }` 判定超时，而成功路径在 `c.Read` 返回后还要经过 `DecodeHello`+版本校验才 `close(helloDone)`。若定时器恰在该微秒级窗口内触发，回调看到 `helloDone` 未关 → 以 1008 `hello_timeout` 关闭一个已在期限内送达合法 Hello 的连接，随后单次语义服务端整体退出。窗口极小且两种结局在边界语义上都说得过去，故列 INFO；如要消歧，可将 `close(helloDone)` 前移到 `c.Read` 成功且首字节校验为 Hello 之后、JSON 解码之前（提前量最大），或显式接受该栅栏语义并在注释中说明（当前注释未提及此窗口）。

### IN-02: `run()` 先 `pty.Start` 后 `net.Listen`——监听失败时子进程被无谓 spawn

**File:** `cmd/wesh/main.go:73-82`
**Issue:** 端口被占是最常见的启动失败形态，此时子进程已被 spawn，随后 `os.Exit` 借进程退出关 master 间接 SIGHUP 收割——依赖内核 hangup 语义而非显式清理；若子进程忽略 SIGHUP 会短暂残留。对调两行（先 Listen 成功再 Start）即可零成本消除，且"listening on"打印时 PTY 尚未创建，启动语义更线性。

### IN-03: 前端 `onmessage` 对 WELCOME/ERROR 载荷 `JSON.parse` 无 try/catch

**File:** `web/src/main.ts:95, 103`
**Issue:** 服务端 bug 或中间设备篡改产生畸形控制帧时，`JSON.parse` 在事件处理器内抛出未捕获异常——WELCOME 场景下 `disableStdin`/`[ro]` 标题静默失效（ro 的 UX 层提示丢失，真边界仍在服务端故无安全后果），ERROR 场景下 `lastError` 不更新导致 onclose 落到兜底文案。各包一层 try/catch 静默跳过即可，与 default 分支的"未知帧静默跳过"纪律一致。

### IN-04: `c.Read` 的消息类型被丢弃——text 帧按 binary 帧同等处理

**File:** `internal/server/server.go:278, 334`
**Issue:** 两处读均写作 `_, data, err := c.Read(ctx)`。README 声明"所有帧为 WebSocket 二进制帧"，但实现接受 text 帧并按同一状态机解析（text 帧载荷首字节为 `'H'` 即可过握手）。无安全后果（两档字节硬顶与帧解析对 text/binary 一视同仁），属宽松实现与文档声明不一致；建议要么校验 `typ != websocket.MessageBinary → 1002`，要么在 README 协议小节注明"消息类型不做强制"。

---

_Reviewed: 2026-08-15T12:29:06Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
