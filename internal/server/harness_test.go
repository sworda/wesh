package server_test

// harness_test.go —— 14-01 D-01 执行机械：newTestServer 小族装配点（收编
// shared/per-client 两装配族为单一 mode 参数化入口；蓝本 PITFALLS :264 +
// 14-RESEARCH §Architecture Patterns 1/2）。CI 结构零改动：go test 单 step 经
// t.Run("mode=shared"/"mode=per-client") 子测试天然双模式覆盖（ci.yml 零 diff）。
//
// 四形态容量依据 = 修订期实证的在册特殊调用点登记（二值签名无法承载，扩展为
// 小族）：
//
//	newTestServer 二值普通形 —— 绝大多数调用点；shared 直传 startTestServerWith
//	  （e2e_test.go，启动期 pty.Start），per-client 直传 startPerClientServer
//	  （perclient_test.go，attach 期 spawn + spawned 追踪 Cleanup）。
//	newTrackedTestServer 三值形（+waitHandlers）—— stderr 捕获类测的同步边
//	  （在册：limits_test.go:132 / auth_e2e_test.go:403 / proxy_e2e_test.go:
//	  46,218,281 / multi_test.go:756 / emptyexit_test.go:250）。
//	newHandleTestServer 四值形（+waitHandlers+srv）—— Shutdown 直调面与白盒
//	  srv 面（在册：health_test.go:251 / shutdown_test.go:107,137；
//	  emptyexit_test.go:322；不需要同步边的调用点忽略 waitHandlers 即可，
//	  wg 惰性无害）。
//	newSessTestServer 三值形（+sessions 访问器）—— PTY 读回面
//	  （resize_arb_test.go:111,144,190,248 消费 ptySize/pollSize；统一
//	  「给我会话集读 winsize」访问器；本 plan 期零消费方——编译期在场，
//	  首个消费方 = 14-04 resize_arb 四子测试）。
//
// 收编论证（D-01）：两族的不对称点 = shared 启动期 spawn（server.New 前直传
// sess）vs per-client attach 期 spawn（server.New(nil)+SpawnFunc）——小族两
// 分支各自直传母本、零装配行为改写，两侧 Cleanup 纪律原样保留（killServer
// e2e_test.go:120-134 泄漏级联减速教训 / spawned 逐一 Kill+Close
// perclient_test.go:88-98）；per-client 族原先缺 handler 追踪变体，由
// startPerClientServerTrackedWithSpawn 姊妹变体（14-01 新增）补齐。未知 mode
// 一律 t.Fatalf（fail-fast，绝不禁默落到任一分支）。

import (
	"testing"

	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// defaultPCSpawnFn 返回小族 per-client 分支的默认 spawnFn——逐字镜像
// startPerClientServer 的默认闭包（perclient_test.go:116-120：cmd/wesh/main.go
// run() 生产闭包的镜像形态，含 startOpts 局部复制 + RemoteUser 赋值——SEC-09
// 注入链：每客户端 remoteUser 不同，opts 必须每次局部构造，共享字段直改即多
// 客户端环境串台）。母本零改动红线下的小段复写：WithSpawn 全形态族
// （startPerClientServerWithSpawn / TrackedWithSpawn）需要 spawnFn 入参，本
// helper 是其 argv 便捷面。
func defaultPCSpawnFn(argv []string) func(cols, rows int, remoteUser string) (*pty.Session, error) {
	return func(cols, rows int, remoteUser string) (*pty.Session, error) {
		opts := pty.StartOptions{Uid: -1, Gid: -1}
		opts.RemoteUser = remoteUser
		return pty.StartWithSize(argv, opts, cols, rows)
	}
}

// newTestServer 二值普通形（小族入口）：shared = startTestServerWith 直传
// （opts 基线 {Writable: true}，mutate 非 nil 时先覆写——与既有测试的
// startTestServer 兼容包装同基线）；per-client = startPerClientServer 直传
// （Cleanup 追踪纪律保留）。
func newTestServer(t *testing.T, mode string, argv []string, mutate func(*server.Options)) (exitCh chan int, wsURL string) {
	t.Helper()
	switch mode {
	case server.SessionModeShared:
		opts := server.Options{Writable: true}
		if mutate != nil {
			mutate(&opts)
		}
		return startTestServerWith(t, argv, opts)
	case server.SessionModePerClient:
		return startPerClientServer(t, argv, mutate)
	default:
		t.Fatalf("unknown mode %q", mode)
		return nil, ""
	}
}

// newTrackedTestServer 三值形（+waitHandlers）：stderr 捕获类测的双模式装配
// 形态——restore() 前需与 handler 内 logEvent 建立同步边。shared =
// startTrackedServerWith 直传；per-client = startPerClientServerTrackedWithSpawn
// （14-01 姊妹变体）直传后丢弃 srv/spawnedSessions。
func newTrackedTestServer(t *testing.T, mode string, argv []string, mutate func(*server.Options)) (exitCh chan int, wsURL string, waitHandlers func()) {
	t.Helper()
	switch mode {
	case server.SessionModeShared:
		opts := server.Options{Writable: true}
		if mutate != nil {
			mutate(&opts)
		}
		return startTrackedServerWith(t, argv, opts)
	case server.SessionModePerClient:
		exitCh, wsURL, _, _, waitHandlers := startPerClientServerTrackedWithSpawn(t, defaultPCSpawnFn(argv), mutate)
		return exitCh, wsURL, waitHandlers
	default:
		t.Fatalf("unknown mode %q", mode)
		return nil, "", nil
	}
}

// newHandleTestServer 四值形（+waitHandlers+srv）：Shutdown 直调面与白盒 srv
// 面的双模式装配形态。shared = startTrackedServerHandle 直传；per-client =
// startPerClientServerTrackedWithSpawn 直传后丢弃 spawnedSessions（srv 保留）。
// 不需要同步边的调用点忽略 waitHandlers 即可——wg 惰性无害（wg.Wait 无人
// 调用即零成本）。
func newHandleTestServer(t *testing.T, mode string, argv []string, mutate func(*server.Options)) (exitCh chan int, wsURL string, waitHandlers func(), srv *server.Server) {
	t.Helper()
	switch mode {
	case server.SessionModeShared:
		opts := server.Options{Writable: true}
		if mutate != nil {
			mutate(&opts)
		}
		return startTrackedServerHandle(t, argv, opts)
	case server.SessionModePerClient:
		exitCh, wsURL, srv, _, waitHandlers := startPerClientServerTrackedWithSpawn(t, defaultPCSpawnFn(argv), mutate)
		return exitCh, wsURL, waitHandlers, srv
	default:
		t.Fatalf("unknown mode %q", mode)
		return nil, "", nil, nil
	}
}

// newSessTestServer 三值形（+sessions 访问器）：PTY 读回面（ptySize/pollSize
// 消费）的双模式统一访问器形态。shared = startResizeServer 直传 + 单例访问器
// 包装（func() []*pty.Session { return []*pty.Session{sess} }）；per-client =
// startPerClientServerWithSpawn 直传取 spawnedSessions（mu 保护拷贝返回）。
// 本 plan 期零消费方（首个消费方 = 14-04 resize_arb 四子测试），编译期在场。
func newSessTestServer(t *testing.T, mode string, argv []string, mutate func(*server.Options)) (exitCh chan int, wsURL string, sessions func() []*pty.Session) {
	t.Helper()
	switch mode {
	case server.SessionModeShared:
		opts := server.Options{Writable: true}
		if mutate != nil {
			mutate(&opts)
		}
		exitCh, wsURL, sess := startResizeServer(t, argv, opts)
		return exitCh, wsURL, func() []*pty.Session { return []*pty.Session{sess} }
	case server.SessionModePerClient:
		exitCh, wsURL, _, spawnedSessions := startPerClientServerWithSpawn(t, defaultPCSpawnFn(argv), mutate)
		return exitCh, wsURL, spawnedSessions
	default:
		t.Fatalf("unknown mode %q", mode)
		return nil, "", nil
	}
}
