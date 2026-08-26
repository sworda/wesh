// Package pty 是 wesh 的 PTY 数据面：spawn（exec 数组 + env 白名单）、
// master 读写、尺寸同步与平台收割，与会话/WS 控制面解耦。
package pty

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
)

// Session 持有一个已 spawn 的子进程及其 PTY master。
type Session struct {
	Cmd    *exec.Cmd
	Master *os.File

	readDone chan struct{} // ReadLoop 退出时关闭（带时限 drain 的等待点，Pitfall 4）

	// fdMu 只护 Resize↔Close：Master.Read/Write 经 os.File 内部 fdmu 与 Close 自同步，
	// 唯独 creack/pty Setsize 裸取 Fd() 不过 fdmu（winsize_unix.go），与 lifecycle 的
	// Drain→Close 并发构成 master fd 竞态（02-02 握手 Resize 落地后 -race 实测命中：
	// fd 关闭后可被内核回收重用，裸 ioctl 可能打到无关 fd）。Read 绝不可入此锁——
	// 读阻塞期间 Close 会被拖死。
	fdMu   sync.Mutex
	closed bool // Close 后置位；Resize 见置位返回 os.ErrClosed（幂等语义）
}

// SpawnCols/SpawnRows 为 PTY spawn 初始尺寸的单一事实源（G-05-1 导出，05-10）：
// StartWithSize 的 Winsize 字面量与服务端零参与者会话尺寸回落值（server 包
// sessionDimsLocked）必须同源——两处各写魔法数会在调整时双写漂移（服务端下发
// 的会话尺寸与真实 PTY 尺寸分叉）。
const (
	SpawnCols = 80
	SpawnRows = 24
)

// StartOptions 承载 pty.Start 的可配面（07-04，OPS-04/OPS-05，D-21/D-24，one-way
// 公开契约，P2 D-15 同纪律）：
//   - Dir = --cwd 子进程工作目录（空串 = 继承服务端 cwd 现状，exec.Cmd 零值语义）；
//   - Term = --term 子进程 TERM（空串 = "xterm-256color" 现状语义——显式空 TERM
//     会使终端能力丢失，--term="" 按未配置处理）；
//   - Uid/Gid = --uid/--gid 降权对（-1 = 不降权现状；Credential 分支与 creack/pty
//     StartWithSize 兼容——它只补 Setsid/Setctty 两字段不覆盖调用方 SysProcAttr，
//     GOMODCACHE start.go:18-25 核实）。
//
// 零值等价纪律：Dir "" + Term "" + Uid -1 + Gid -1 时行为与选项化前逐字节一致
// （TestStartZeroValueParity 锁定）。
type StartOptions struct {
	Dir  string
	Term string
	Uid  int
	Gid  int
}

// Start 以 exec 数组形式 spawn argv（绝不经 shell，D-02/D-15），替换式注入 env
// 白名单（SEC-06），初始尺寸 80x24（SpawnCols×SpawnRows；首个客户端 RESIZE 到达
// 即纠正，PITFALLS C10 首帧窗口可接受）。可配面见 StartOptions（07-04 选项化）。
func Start(argv []string, opts StartOptions) (*Session, error) {
	if len(argv) == 0 {
		return nil, errors.New("pty: empty argv")
	}
	cmd := exec.Command(argv[0], argv[1:]...)   // exec 数组，绝不经 shell
	cmd.Env = whitelistEnv(opts.Term, opts.Uid) // SEC-06：替换式注入，非追加
	// 不设 cmd.Stdin/Stdout/Stderr（StartWithAttrs 仅在三者全 nil 时接管 tty）。
	cmd.Dir = opts.Dir // D-21（07-04 已兑现）：--cwd；空串 = 继承服务端 cwd（exec.Cmd 零值语义）
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: SpawnRows, Cols: SpawnCols})
	if err != nil {
		// creack/pty 失败路径只关自己打开的 fd（Pitfall 1，实测 fd 0/1/2 完好）；
		// 本包遵守"只关成功打开且登记在册的 fd"纪律。
		return nil, err
	}
	return &Session{Cmd: cmd, Master: master, readDone: make(chan struct{})}, nil
}

// whitelistEnv 构造子进程环境白名单：TERM/COLORTERM 固定；PATH/HOME/USER/LOGNAME/SHELL
// 按名继承；LANG=/LC_ 前缀继承；其余一律丢弃（SEC-06，D-15）。
// 替换式注入——严禁把 os.Environ() 全量追加进来（ttyd pty.c:441-444 同款泄露）。
// 07-04（D-21/D-25）：term 参数化 TERM= 行（空串 = "xterm-256color" 现状语义）；
// uid 为 --uid 降权目标（-1 = 不降权按名继承现状；>=0 时按目标 uid passwd 条目
// 改写 HOME/USER/LOGNAME 三键，D-25 挂点在下方继承循环内）。
func whitelistEnv(term string, uid int) []string {
	if term == "" {
		term = "xterm-256color" // D-21：空 = 默认现状语义（显式空 TERM 防能力丢失）
	}
	env := []string{
		"TERM=" + term, // wesh 前端真实能力默认值；--term 可配（D-21）
		"COLORTERM=truecolor",
	}
	// 仅继承非机密必需项；LANG/LC_* 前缀匹配继承；其余一律丢弃。
	// 空串值跳过不继承——否则 PATH="" 会与下方回退项并存，getenv 取首个
	// 匹配（"PATH=" 在前）使回退失效，子进程 shell 按空 PATH 找不到命令。
	for _, k := range []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL"} {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			env = append(env, k+"="+v)
		}
	}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "LANG=") || strings.HasPrefix(kv, "LC_") {
			env = append(env, kv)
		}
	}
	if v, ok := os.LookupEnv("PATH"); !ok || v == "" {
		env = append(env, "PATH=/usr/local/bin:/usr/bin:/bin")
	}
	return env
}
