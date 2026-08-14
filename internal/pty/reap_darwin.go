//go:build darwin

package pty

// darwin 收割模型（D-14）：进程级共享 kqueue watcher（EVFILT_PROC/NOTE_EXIT，
// EV_ADD|EV_ONESHOT 注册）只做"早知"，收割仍由 cmd.Wait() 完成——唯一收割者、
// 退出码完整。
//
// 纪律（RESEARCH 修正决议，行 488）：禁手写 pidfd_open / SIGCHLD+WNOHANG 手动
// reap——两者都会与 Wait 争收割权，丢退出码。
//
// 兜底预案（RESEARCH Open Questions Q1）：若竞态测试 TestKqueueExitZombieRace
// 裁决 kqueue 不补发僵尸进程事件，本文件的 awaitExit 退化为直接 cmd.Wait()
// （每会话一个阻塞 goroutine，v1 单会话规模可接受），watcher 代码以 build tag
// 保留，待 Phase 5 多会话时重估。

import (
	"os/exec"
	"sync"

	"golang.org/x/sys/unix"
)

// exitWatcher 进程级共享：一个 kqueue fd、一个 loop goroutine，N 会话共用
// （零每会话线程，D-14）。
type exitWatcher struct {
	kq   int
	mu   sync.Mutex
	subs map[int]chan<- struct{} // pid -> notify
}

func newExitWatcher() (*exitWatcher, error) {
	kq, err := unix.Kqueue()
	if err != nil {
		return nil, err
	}
	w := &exitWatcher{kq: kq, subs: make(map[int]chan<- struct{})}
	go w.loop()
	return w, nil
}

func (w *exitWatcher) watch(pid int) (<-chan struct{}, error) {
	ch := make(chan struct{}, 1)
	w.mu.Lock()
	w.subs[pid] = ch
	w.mu.Unlock()
	// EV_ADD 注册；EV_ONESHOT 触发一次即自动注销
	ev := []unix.Kevent_t{{
		Ident:  uint64(pid),
		Filter: unix.EVFILT_PROC,
		Flags:  unix.EV_ADD | unix.EV_ONESHOT,
		Fflags: unix.NOTE_EXIT | unix.NOTE_EXITSTATUS,
	}}
	if _, err := unix.Kevent(w.kq, ev, nil, nil); err != nil { // 非阻塞注册
		// 注册失败须摘除已登记的订阅，避免 subs 泄漏（Rule 2 自动修复）
		w.mu.Lock()
		delete(w.subs, pid)
		w.mu.Unlock()
		return nil, err
	}
	return ch, nil
}

func (w *exitWatcher) loop() {
	events := make([]unix.Kevent_t, 8)
	for {
		n, err := unix.Kevent(w.kq, nil, events, nil) // 阻塞取事件
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return // TODO(Phase 8): 接 slog；Phase 1 进程级致命即可
		}
		for i := 0; i < n; i++ {
			if events[i].Fflags&unix.NOTE_EXIT == 0 {
				continue
			}
			pid := int(events[i].Ident)
			w.mu.Lock()
			ch, ok := w.subs[pid]
			delete(w.subs, pid)
			w.mu.Unlock()
			if ok {
				ch <- struct{}{}
			}
		}
	}
}

// 包级单例 watcher（sync.Once 懒初始化）；初始化失败时 awaitExit 退化为直接
// cmd.Wait()（兜底不致命）。
var (
	watcherOnce sync.Once
	watcher     *exitWatcher
	watcherErr  error
)

func sharedExitWatcher() (*exitWatcher, error) {
	watcherOnce.Do(func() {
		watcher, watcherErr = newExitWatcher()
	})
	return watcher, watcherErr
}

// awaitExit 与 reap_linux.go 签名统一（PATTERNS 注意 4——调用点零平台分支）。
// <-exited 后 return cmd.Wait()：Wait 是唯一收割者（wait4 收割 +
// *exec.ExitError 退出码完整），watcher 只做"早知"。watch 注册失败时同样
// 退化为直接 cmd.Wait()。
func awaitExit(cmd *exec.Cmd) error {
	w, err := sharedExitWatcher()
	if err != nil {
		return cmd.Wait()
	}
	exited, err := w.watch(cmd.Process.Pid)
	if err != nil {
		return cmd.Wait()
	}
	<-exited
	return cmd.Wait()
}
