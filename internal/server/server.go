// Package server 提供 HTTP + WS 网关、数据泵与 Phase 1 单次语义生命周期：
// 任意终结路径（子进程退出 / WS 断开）都使服务端经 exitf 整体退出（D-10/D-11）。
package server

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/web"
)

// Server 持有会话、attach 原子门与生命周期收口件。
// exitf 由 main 注入 os.Exit、测试注入捕获桩——生命周期必须可测，这是硬约束。
type Server struct {
	sess  *pty.Session
	exitf func(code int)

	attached atomic.Bool                    // D-09：单客户端原子门
	conn     atomic.Pointer[websocket.Conn] // 当前已 attach 的 WS 连接（onChunk 写端 / 1000 关闭用）
	frame    []byte                         // OUTPUT 组帧缓冲（仅 ReadLoop 单 goroutine 经 onChunk 访问，无竞争）
	termOnce sync.Once                      // 两条终结路径收口，exitf 只触发一次
}

// New 装配服务端并钉死两个 goroutine 的启动点：
//   - sess.ReadLoop：自装配起持续 drain master（D-12），attach 前输出直接丢弃，
//     防 64KiB PTY 内核缓冲填满导致子进程写阻塞；attach 路径内不得再新建读循环。
//   - lifecycle：sess.Wait → 带时限 drain → 1000 关闭当前客户端 → exitf（D-10 触发源）。
func New(sess *pty.Session, exitf func(int)) *Server {
	s := &Server{
		sess:  sess,
		exitf: exitf,
		frame: make([]byte, 1+32*1024),
	}
	s.frame[0] = proto.Output
	go sess.ReadLoop(s.onChunk)
	go s.lifecycle()
	return s
}

// Handler 挂两条路由：/ 走 go:embed 静态伺服，/ws 走 Attach。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	wh, err := web.Handler()
	if err != nil {
		// fs.Sub 仅在内嵌 FS 缺 dist 时失败（编译期 go:embed 已保证存在）；防御性 500。
		wh = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "embedded assets unavailable", http.StatusInternalServerError)
		})
	}
	mux.Handle("/", wh)
	mux.HandleFunc("/ws", s.Attach)
	return mux
}

// Attach 是 /ws 的 attach handler（PATTERNS 称 serveWS）。
func (s *Server) Attach(w http.ResponseWriter, r *http.Request) {
	// D-09：第二连接在 Accept 之前以 HTTP 409 拒绝。
	if !s.attached.CompareAndSwap(false, true) {
		http.Error(w, "another client is already attached", http.StatusConflict)
		return
	}
	// AcceptOptions 空字面量：Subprotocols 留空（wesh.v1 属 Phase 2）；压缩默认禁用
	// （终端高熵数据无收益）；不跳过 Origin 校验——库默认同源校验（同 Host 放行、
	// 跨源拒绝，D-17；Phase 1 页面与 WS 同 server 同 origin，默认即安全）。
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		s.attached.Store(false) // Accept 失败即未 attach，释放门位允许后续客户端
		return                  // Accept 失败已自动写 HTTP 错误响应
	}
	defer c.CloseNow()
	s.conn.Store(c)
	defer s.conn.Store(nil)
	// SetReadLimit 用库默认 32768 即预认证基线（read.go:107），超限库自动 1009。
	// ctx 由 context.Background() 派生——禁止 r.Context()（hijack 后行为意外，官方 README 明示）。
	ctx := context.Background()

	// C→S：单 reader 循环（c.Read 不可并发，Pitfall 7）。
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			// 对端关闭（errors.As 可取出 CloseError）与网络断开同等处理：
			// Phase 1 单次语义，任何 reader 终结都走 D-11 路径。
			s.wsDisconnected()
			return
		}
		if len(data) == 0 {
			continue
		}
		switch data[0] {
		case proto.Input:
			s.sess.Master.Write(data[1:])
		case proto.Resize:
			// JSON 解码失败静默丢弃（不关连接）；成功时已钳制 [1,1000]（D-16）
			if cols, rows, ok := proto.DecodeResize(data[1:]); ok {
				s.sess.Resize(cols, rows)
			}
		default:
			c.Close(websocket.StatusProtocolError, "unknown frame type") // 1002，协议演化无歧义
		}
	}
}

// onChunk 是 S→C 数据泵（ReadLoop 回调，独占 WS 写端）：
// 未 attach 期间直接丢弃（D-12 drain 语义）；已 attach 组 OUTPUT 帧写 WS。
// 仅由 ReadLoop 单 goroutine 调用，故复用 s.frame 无竞争。
func (s *Server) onChunk(chunk []byte) {
	c := s.conn.Load()
	if c == nil {
		return // D-12：attach 前输出丢弃，防 PTY 内核缓冲写阻塞
	}
	n := copy(s.frame[1:], chunk)
	if err := c.Write(context.Background(), websocket.MessageBinary, s.frame[:1+n]); err != nil {
		return // 写失败（连接已死）：终结由 reader 路径收口（D-11），本块丢弃
	}
}

// lifecycle 是 D-10 路径触发源：子进程退出 → 带时限 drain（Pitfall 4）→
// 当前已 attach 客户端收 1000 正常关闭帧 → exitf（退出码 = 子进程退出码）。
func (s *Server) lifecycle() {
	err := s.sess.Wait()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	s.sess.Drain(200 * time.Millisecond)
	if c := s.conn.Load(); c != nil {
		c.Close(websocket.StatusNormalClosure, "") // D-10：1000
	}
	s.terminate(false, code) // 子进程已退出，无需 SIGHUP
}

// wsDisconnected 是 D-11 路径：WS 断开（reader 返回错误）→ SIGHUP 子进程进程组 → exitf(0)。
func (s *Server) wsDisconnected() {
	s.terminate(true, 0)
}

// terminate 以 sync.Once 收口两条终结路径，exitf 只触发一次。
// sighup 为真时先 SIGHUP 子进程进程组：负 pid = 进程组；setsid 使子进程为组长，
// pgid = 子进程 pid（D-11）。Start 成功后 Cmd.Process 必非 nil。
func (s *Server) terminate(sighup bool, code int) {
	s.termOnce.Do(func() {
		sess := s.sess
		if sighup {
			syscall.Kill(-sess.Cmd.Process.Pid, syscall.SIGHUP)
		}
		s.exitf(code)
	})
}
