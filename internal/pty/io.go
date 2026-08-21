package pty

import (
	"os"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// ReadLoop 以 32KiB 缓冲循环读 master：n>0 回调 onChunk，任何 read 错误（含 io.EOF
// 与 Linux EIO）统一归一为"输出终结"即 return（Pitfall 3，禁止 err == io.EOF 单判）。
//
// onChunk 在读循环 goroutine 内同步调用、复用底层缓冲——回调方如需跨帧持有须自行拷贝。
// ReadLoop 必须恰好调用一次（server.New 装配时启动并持续运行，D-12 drain）。
func (s *Session) ReadLoop(onChunk func([]byte)) {
	defer close(s.readDone)
	buf := make([]byte, 32*1024)
	for {
		n, err := s.Master.Read(buf)
		if n > 0 {
			onChunk(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// Resize 经 TIOCSWINSZ 同步 PTY 尺寸；ws_xpixel/ws_ypixel 置 0 是 ttyd 与
// creack/pty 共同实践。cols/rows 已由 proto.DecodeResize 钳制到 [1,1000]（D-16），
// uint16 转换安全。经 fdMu 与 Close 互斥（见 Session.fdMu 注释）；Close 后调用
// 返回 os.ErrClosed（Attach 读循环忽略 Resize 错误，语义不变）。
func (s *Session) Resize(cols, rows int) error {
	s.fdMu.Lock()
	defer s.fdMu.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	return pty.Setsize(s.Master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// SignalForegroundGroup 向 PTY 前台进程组显式发一次 SIGWINCH（D-11 新客强制重绘）：
// TIOCGPGRP 取前台 pgid → kill(-pgid, SIGWINCH)（负 pid = 进程组）。必须显式发且
// 与仲裁 resize 是否发生无关——Linux 同尺寸 TIOCSWINSZ 不发 SIGWINCH（P5-3 本机
// 实证），新客 attach 若无显式信号，vim/htop 等全屏程序不重绘即黑屏。TIOCGPGRP
// 失败/无前台进程组/会话已 closed 时静默降级（D-11 授权）；重复 SIGWINCH 无害
// （终端应用必须容忍伪信号）。fdMu 持锁范围与 Resize 同款（spawn.go:22：Read/Write
// 经 os.File 内部 fdmu 自同步、绝不可入此锁）。
func (s *Session) SignalForegroundGroup() {
	s.fdMu.Lock()
	defer s.fdMu.Unlock()
	if s.closed {
		return
	}
	pgid, err := unix.IoctlGetInt(int(s.Master.Fd()), unix.TIOCGPGRP)
	if err != nil || pgid <= 0 {
		return // 静默降级（D-11 授权）
	}
	_ = unix.Kill(-pgid, unix.SIGWINCH) // 负 pid = 进程组；失败静默
}

// Close 关闭 master（有效指针判空，非零值推断——Pitfall 1"只关成功打开且登记在册的 fd"）。
// 经 fdMu 与 Resize 互斥并置 closed——幂等，重复关闭安全。
func (s *Session) Close() error {
	s.fdMu.Lock()
	defer s.fdMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.Master != nil {
		return s.Master.Close()
	}
	return nil
}

// Wait 阻塞等待子进程退出，平台收割实现见 reap_*.go（唯一收割者，退出码完整）。
func (s *Session) Wait() error {
	return awaitExit(s.Cmd)
}

// Drain 带时限 drain（Pitfall 4）：Wait 返回后给读循环最多 d 等 EOF/EIO，到点无条件
// Close(master)——残留孙进程下次读写 slave 得 EIO 自然消亡。
func (s *Session) Drain(d time.Duration) {
	select {
	case <-s.readDone:
	case <-time.After(d):
	}
	s.Close()
}
