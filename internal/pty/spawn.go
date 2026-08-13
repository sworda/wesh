// Package pty 是 wesh 的 PTY 数据面：spawn（exec 数组 + env 白名单）、
// master 读写、尺寸同步与平台收割，与会话/WS 控制面解耦。
package pty

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)

// Session 持有一个已 spawn 的子进程及其 PTY master。
type Session struct {
	Cmd    *exec.Cmd
	Master *os.File

	readDone chan struct{} // ReadLoop 退出时关闭（带时限 drain 的等待点，Pitfall 4）
}

// Start 以 exec 数组形式 spawn argv（绝不经 shell，D-02/D-15），替换式注入 env
// 白名单（SEC-06），初始尺寸 80x24（首个客户端 RESIZE 到达即纠正，PITFALLS C10
// 首帧窗口可接受）。
func Start(argv []string) (*Session, error) {
	if len(argv) == 0 {
		return nil, errors.New("pty: empty argv")
	}
	cmd := exec.Command(argv[0], argv[1:]...) // exec 数组，绝不经 shell
	cmd.Env = whitelistEnv()                  // SEC-06：替换式注入，非追加
	// 不设 cmd.Stdin/Stdout/Stderr（StartWithAttrs 仅在三者全 nil 时接管 tty）与
	// cmd.Dir（Phase 1 继承服务端 cwd；OPS-04 可配留 Phase 7）。
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
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
func whitelistEnv() []string {
	env := []string{
		"TERM=xterm-256color", // wesh 前端真实能力；OPS-04 可配置留到 Phase 7
		"COLORTERM=truecolor",
	}
	// 仅继承非机密必需项；LANG/LC_* 前缀匹配继承；其余一律丢弃
	for _, k := range []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "LANG=") || strings.HasPrefix(kv, "LC_") {
			env = append(env, kv)
		}
	}
	if _, ok := os.LookupEnv("PATH"); !ok {
		env = append(env, "PATH=/usr/local/bin:/usr/bin:/bin")
	}
	return env
}
