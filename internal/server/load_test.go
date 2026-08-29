//go:build load

// load_test.go —— 09-06 D-11/D-12 负载矩阵黑盒测试（P2/P5/P6 挂账默认参数的
// 标定验证通道；README §默认参数与 Phase 9 标定 231-243 既定方法论实例化）。
//
// 运行（手动，build tag 隔离不进常规 CI——D-11 裁决）：
//
//	go test -tags=load -count=1 -timeout=30m ./internal/server/ -v
//
// 首行硬纪律：//go:build load 必须是文件首行（否则常规 CI 捡起本文件的重型
// 测试——RESEARCH Anti-Pattern）；package server_test 外部包形态（e2e_test.go/
// limits_test.go/metrics_test.go 全仓统一）。
//
// 标定纪律（D-12）：验证为主、证伪才改——默认验证现值成立（合法慢端零误踢 +
// 内存上界成立 + 信用门开闭频率可接受），数据证伪才改常量默认值（不动 flag
// 面）。每格产出 LOADDATA 行（clients/profile/slowlink/kicks/
// gate_transitions/outbox_max/alloc_peak/alloc_base/dur_ms）——09-09 D-13
// README 标定表回填的直接数据源。
//
// 夹具纪律（slowclient_test.go 头注释戒律沿用）：stall 端 dialHello 后绝不
// Read；客户端 Read 永不带 deadline ctx（goroutine + 缓冲 channel + select
// time.After 竞速形态）；loopback 单连接最坏吸收 ≈ wmem 4MiB + rmem 6MiB
//（本机实测），stall/限速施压格洪水量级 ≫10MiB。每格独立
// startTrackedServerWith + t.Cleanup(killServer) 收口（e2e_test.go:120-134
// 泄漏教训先例）。数据源单侧 = /metrics 黑盒 scrape（metrics_test.go
// getMetrics/metricSample 先例）+ 进程内 runtime 观测（D-11 in-process 装配
// 依据：测试进程即服务端进程）。
package server_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// ====== 夹具层 ======

// loadTailKeep 为 drain 统计保留的流尾字节数（提取末位 seq 字段的采样窗——
// 负载纪律：32 端全量缓存超测试进程内存预算，一致性采样靠字节数 + 末位字段）。
const loadTailKeep = 128

// loadDrain 为单端 drain 终态统计：OUTPUT 载荷字节数/帧数/流尾采样/终结错误。
type loadDrain struct {
	bytes  int64
	frames int64
	tail   []byte
	err    error
}

// note 记账一条 OUTPUT 载荷（帧类型字节不计入字节数——与 wesh_pty_output_bytes_total
// 口径对齐）。
func (r *loadDrain) note(data []byte) {
	r.bytes += int64(len(data) - 1)
	r.frames++
	payload := data[1:]
	if len(payload) > loadTailKeep {
		payload = payload[len(payload)-loadTailKeep:]
	}
	r.tail = append(r.tail[:0], payload...)
}

// drainClient 持续读 conn 至终结（只计数不缓存）。Read 永不带 deadline ctx
//（Pitfall 2 竞速形态）——调用方以 select time.After 收口（awaitDrain）。
func drainClient(c *websocket.Conn) <-chan loadDrain {
	ch := make(chan loadDrain, 1)
	go func() {
		var r loadDrain
		for {
			_, data, err := c.Read(context.Background())
			if err != nil {
				r.err = err
				ch <- r
				return
			}
			if len(data) > 0 && data[0] == proto.Output {
				r.note(data)
			}
		}
	}()
	return ch
}

// drainRateLimited 限速读者（D-12 断言一的合法慢端形态）：应用层按消息字节数
// 折算停顿，drain 速率 ≈ rateBps。注意 loopback 内核吸收带（wmem+rmem ≈
// 10MiB）使服务端 outbox 只在吸收带填满后才承压——本形态用于「drain ≈ 产出」
// 的合法慢端零误踢验证，outbox 承压面由 TestLoadGateTransitions 覆盖。
func drainRateLimited(c *websocket.Conn, rateBps int64) <-chan loadDrain {
	ch := make(chan loadDrain, 1)
	go func() {
		var r loadDrain
		for {
			_, data, err := c.Read(context.Background())
			if err != nil {
				r.err = err
				ch <- r
				return
			}
			if len(data) > 0 && data[0] == proto.Output {
				r.note(data)
			}
			if d := time.Duration(int64(time.Second) * int64(len(data)) / rateBps); d > 0 {
				time.Sleep(d)
			}
		}
	}()
	return ch
}

// awaitDrain 以竞速形态收口 drain（Read 无 deadline 纪律的调用侧配对）。
func awaitDrain(t *testing.T, ch <-chan loadDrain, timeout time.Duration, who string) loadDrain {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(timeout):
		t.Fatalf("%s drain 未在 %v 内终结", who, timeout)
		return loadDrain{}
	}
}

// assertClosed1000 断言 drain 以 CloseError 1000 终结（子进程退出 lifecycle
// 广播序列——EXIT 帧被 OUTPUT 过滤忽略，终结码是唯一的收口证据形态）。
func assertClosed1000(t *testing.T, r loadDrain, who string) {
	t.Helper()
	var ce websocket.CloseError
	if !errors.As(r.err, &ce) || ce.Code != websocket.StatusNormalClosure {
		t.Fatalf("%s read 终结于 %v（bytes=%d），want CloseError 1000（子进程退出广播）", who, r.err, r.bytes)
	}
}

// dialLoadClient 负载客户端工厂：dialHello 握手 + SetReadLimit 4MiB（writer
// 合并段超 Go 客户端库默认 32KiB 触发 1009 的既有先例——05-02 实测命中）。
func dialLoadClient(t *testing.T, ctx context.Context, wsURL string) *websocket.Conn {
	t.Helper()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	c.SetReadLimit(4 * 1024 * 1024)
	return c
}

// loadMetricValue 在 exposition body 中精确查找 series 样本值（metricSample
// 的无 t 变体——采样器 goroutine 内不得调 t.Fatal）。
func loadMetricValue(body, name string) (int64, bool) {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, name+" ") {
			v, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, name)), 10, 64)
			if err != nil {
				return 0, false
			}
			return v, true
		}
	}
	return 0, false
}

// scrapePeakSampler 周期黑盒 scrape /metrics 并跟踪指定 series 的峰值
//（wesh_outbox_depth_bytes_max 的快照是瞬态值——格内峰值须采样获取）。
// 尽力而为：单次 scrape 失败跳过该拍，不致命。
func scrapePeakSampler(url string, names []string, interval time.Duration, stop <-chan struct{}) <-chan map[string]int64 {
	ch := make(chan map[string]int64, 1)
	go func() {
		peak := make(map[string]int64, len(names))
		tk := time.NewTicker(interval)
		defer tk.Stop()
		poll := func() {
			resp, err := http.Get(url)
			if err != nil {
				return
			}
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil || resp.StatusCode != http.StatusOK {
				return
			}
			for _, n := range names {
				if v, ok := loadMetricValue(string(b), n); ok && v > peak[n] {
					peak[n] = v
				}
			}
		}
		for {
			select {
			case <-stop:
				ch <- peak
				return
			case <-tk.C:
				poll()
			}
		}
	}()
	return ch
}

// allocPeakSampler 进程内 100ms 采样 runtime.ReadMemStats Alloc 峰值（D-11
// in-process 装配依据——测试进程即服务端进程，内存断言直接可达）。
func allocPeakSampler(stop <-chan struct{}) <-chan uint64 {
	ch := make(chan uint64, 1)
	go func() {
		var peak uint64
		tk := time.NewTicker(100 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-stop:
				ch <- peak
				return
			case <-tk.C:
				if a := readAlloc(); a > peak {
					peak = a
				}
			}
		}
	}()
	return ch
}

func readAlloc() uint64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.Alloc
}

// loadFloodLast 洪水末位序号（平台分支同 seqFlood 纪律：darwin BSD seq 默认
// %g 仅 6 位有效数字，≥1e6 转科学计数法破坏末位字段断言——压 999999；load
// 测试主口径为 Linux 手动跑）。seq 1 N 总量 ≈ 33.8MB（N=4000000，含 ONLCR
// 膨胀）——≫ loopback 最坏吸收 ~10MiB 纪律（slowclient_test.go 头注释）。
func loadFloodLast() int {
	if runtime.GOOS == "darwin" {
		return 999999
	}
	return 4000000
}

// gatedFloodArgv 触发式洪水 argv：bash 先从 stdin 读一行（全部客户端 attach
// 完成后由客户端 INPUT 触发），消除 pre-attach drain 的不确定量——每端收流
// 与 wesh_pty_output_bytes_total 的字节级对照由此精确（D-12 数据源精确性）。
// 尾闸 sleep 1：子进程退出即触发 EXIT+1000 广播（绕过 outbox 直写 wire），
// 无尾闸则活跃读格在 32 端 CPU 竞争下 observed 末段 outbox 残余随关闭丢弃
//（首跑实测 client 4 缺 22428/34888899 字节）——停 1s 让 writer 腾空 outbox
// 后广播，严格字节相等断言结构性成立。
func gatedFloodArgv(last int) []string {
	return []string{"bash", "-c", fmt.Sprintf("read x; seq 1 %d; sleep 1", last)}
}

// loadSamplers 每格统一开/收的采样器组（Alloc 峰值 + /metrics outbox 峰值）。
type loadSamplers struct {
	stopAlloc  chan struct{}
	stopScrape chan struct{}
	allocCh    <-chan uint64
	scrapeCh   <-chan map[string]int64
}

func startLoadSamplers(metricsURL string) *loadSamplers {
	s := &loadSamplers{
		stopAlloc:  make(chan struct{}),
		stopScrape: make(chan struct{}),
	}
	s.allocCh = allocPeakSampler(s.stopAlloc)
	s.scrapeCh = scrapePeakSampler(metricsURL, []string{"wesh_outbox_depth_bytes_max"}, 250*time.Millisecond, s.stopScrape)
	return s
}

func (s *loadSamplers) stop() (allocPeak uint64, outboxMax int64) {
	close(s.stopAlloc)
	close(s.stopScrape)
	allocPeak = <-s.allocCh
	peaks := <-s.scrapeCh
	outboxMax = peaks["wesh_outbox_depth_bytes_max"]
	return allocPeak, outboxMax
}

// ====== 矩阵测试族 ======

// TestLoadFanoutMatrix（矩阵格族：客户端数 {1,4,16,32} × seq 洪水 × 全活跃读，
// README 231-243 矩阵形状实例化）：全端收流一致（字节数逐端相等 + 末位字段到
// 洪水末尾的采样校验）+ kicks 精确 ==0（D-12 断言一的活跃读面）+ 放大比
// wesh_ws_sent_bytes_total ≥ N×wesh_pty_output_bytes_total（metrics_test.go
// TestMetricsValues 先例的负载形态）。
//
// WritePolicy=all 形态理由：全员 rw 使离群误踢路径武装化（rw + 出宽限 + 存在
// 未 blocked 可写端 → 踢），kicks==0 在该分工表形态下方具判别力；owner 默认
// 下后续端为 ro，ro 满即踢的分工不同源。
func TestLoadFanoutMatrix(t *testing.T) {
	last := loadFloodLast()
	for _, n := range []int{1, 4, 16, 32} {
		n := n
		t.Run(fmt.Sprintf("clients_%d", n), func(t *testing.T) {
			_, wsURL, _ := startTrackedServerWith(t, gatedFloodArgv(last), server.Options{
				Writable:    true,
				WritePolicy: "all",
			})
			base := httpBaseOf(wsURL)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			conns := make([]*websocket.Conn, 0, n)
			drains := make([]<-chan loadDrain, 0, n)
			for i := 0; i < n; i++ {
				c := dialLoadClient(t, ctx, wsURL)
				conns = append(conns, c)
				drains = append(drains, drainClient(c))
			}
			t.Cleanup(func() {
				for _, c := range conns {
					c.CloseNow()
				}
			})

			runtime.GC()
			time.Sleep(100 * time.Millisecond)
			allocBase := readAlloc()
			smp := startLoadSamplers(base + "/metrics")

			// 触发洪水：1 端 INPUT 一行 → bash read 放行 → seq 起洪。
			start := time.Now()
			if err := conns[0].Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
				t.Fatalf("write 触发 INPUT: %v", err)
			}
			results := make([]loadDrain, n)
			for i := range drains {
				results[i] = awaitDrain(t, drains[i], 240*time.Second, fmt.Sprintf("client %d", i))
			}
			dur := time.Since(start)
			allocPeak, outboxMax := smp.stop()

			for i, r := range results {
				assertClosed1000(t, r, fmt.Sprintf("client %d", i))
			}
			// 全端收流一致（采样校验）：字节数逐端相等 + 末位字段 == 洪水末位。
			for i := 1; i < n; i++ {
				if results[i].bytes != results[0].bytes {
					t.Fatalf("client %d 收流 = %d 字节, want == client 0 的 %d（fan-out 全端一致）", i, results[i].bytes, results[0].bytes)
				}
			}
			wantTail := strconv.Itoa(last)
			for i, r := range results {
				fields := strings.Fields(string(r.tail))
				if len(fields) == 0 || fields[len(fields)-1] != wantTail {
					t.Fatalf("client %d 流尾末位字段 = %v, want %d（收流完整到洪水末尾）", i, fields, last)
				}
			}

			body := getMetrics(t, base+"/metrics")
			kicks, _ := metricSample(t, body, "wesh_clients_kicked_total")
			gateT, _ := metricSample(t, body, "wesh_credit_gate_transitions_total")
			ptyOut, _ := metricSample(t, body, "wesh_pty_output_bytes_total")
			wsSent, _ := metricSample(t, body, "wesh_ws_sent_bytes_total")
			memAlloc, _ := metricSample(t, body, "wesh_mem_alloc_bytes")
			if kicks != 0 {
				t.Fatalf("wesh_clients_kicked_total = %d, want 精确 0（活跃读矩阵零误踢）", kicks)
			}
			if wsSent < int64(n)*ptyOut {
				t.Fatalf("放大比不成立：wesh_ws_sent_bytes_total = %d, want ≥ %d×wesh_pty_output_bytes_total(%d)", wsSent, n, ptyOut)
			}
			t.Logf("LOADDATA cell=fanout clients=%d profile=seq_flood(last=%d) slowlink=none kicks=%d gate_transitions=%d outbox_max=%d alloc_peak=%d alloc_base=%d mem_alloc_end=%d bytes_per_client=%d dur_ms=%d",
				n, last, kicks, gateT, outboxMax, allocPeak, allocBase, memAlloc, results[0].bytes, dur.Milliseconds())
		})
	}
}

// TestLoadLegitSlowReaderZeroKick（D-12 断言一验收首要格）：滴漏产出（1KiB/5ms
// ≈ 205KB/s）+ 每 200 拍 128KiB 突发抖动（< 默认 outbox 容量 512KiB）× 限速
// 合法读者（drain 400KB/s ≈ 平均产出 ~330KB/s）。同格快读者武装离群误踢路径
//（限速端 outbox 写满 + 快端未 blocked → 踢）；全程 wesh_clients_kicked_total
// 精确不增 ==0 且限速端收流与快端逐字节相等（零丢帧——trySend 失败只走
// 踢出/信用，绝无静默丢帧的推论断言）。
func TestLoadLegitSlowReaderZeroKick(t *testing.T) {
	// 尾闸 sleep 1 同 gatedFloodArgv 注释纪律（广播前腾空 outbox，逐字节相等断言前提）。
	argv := []string{"bash", "-c", `i=0; while [ $i -lt 2000 ]; do printf '%1024d\n' "$i"; if [ $((i % 200)) -eq 99 ]; then head -c 131072 /dev/zero | tr '\0' 'B'; fi; i=$((i+1)); sleep 0.005; done; sleep 1`}
	_, wsURL, _ := startTrackedServerWith(t, argv, server.Options{
		Writable:    true,
		WritePolicy: "all",
	})
	base := httpBaseOf(wsURL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fast := dialLoadClient(t, ctx, wsURL)
	slow := dialLoadClient(t, ctx, wsURL)
	t.Cleanup(func() { fast.CloseNow(); slow.CloseNow() })
	fastDrain := drainClient(fast)
	slowDrain := drainRateLimited(slow, 400*1024)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	allocBase := readAlloc()
	smp := startLoadSamplers(base + "/metrics")
	start := time.Now()

	fastR := awaitDrain(t, fastDrain, 90*time.Second, "fast reader")
	slowR := awaitDrain(t, slowDrain, 90*time.Second, "rate-limited reader (400KB/s)")
	dur := time.Since(start)
	allocPeak, outboxMax := smp.stop()

	assertClosed1000(t, fastR, "fast reader")
	assertClosed1000(t, slowR, "rate-limited reader (400KB/s)")
	if fastR.bytes != slowR.bytes {
		t.Fatalf("限速读者收流 = %d 字节, want == 快读者 %d（零丢帧逐字节相等）", slowR.bytes, fastR.bytes)
	}

	body := getMetrics(t, base+"/metrics")
	kicks, _ := metricSample(t, body, "wesh_clients_kicked_total")
	gateT, _ := metricSample(t, body, "wesh_credit_gate_transitions_total")
	if kicks != 0 {
		t.Fatalf("wesh_clients_kicked_total = %d, want 精确 0（合法慢端零误踢，D-12 验收首要）", kicks)
	}
	t.Logf("LOADDATA cell=legit_slow clients=2 profile=drip_205KBps+burst_128KiB slowlink=rate_limited_400KBps kicks=%d gate_transitions=%d outbox_max=%d alloc_peak=%d alloc_base=%d bytes_per_client=%d dur_ms=%d",
		kicks, gateT, outboxMax, allocPeak, allocBase, fastR.bytes, dur.Milliseconds())
}

// TestLoadMemoryBound（D-12 断言二）：32 端 seq 洪水下 Alloc 峰值 ≤ 4× 账面
// 最坏（32×512KiB outbox ≈ 16MiB + 共享帧 → 防失控上界 64MiB），矩阵结束
// runtime.GC() 后回基线 ±50%；wesh_mem_alloc_bytes /
// wesh_outbox_depth_bytes_max 同步观测记录（LOADDATA 行）。
func TestLoadMemoryBound(t *testing.T) {
	const n = 32
	last := loadFloodLast()
	_, wsURL, _ := startTrackedServerWith(t, gatedFloodArgv(last), server.Options{
		Writable:    true,
		WritePolicy: "all",
	})
	base := httpBaseOf(wsURL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conns := make([]*websocket.Conn, 0, n)
	drains := make([]<-chan loadDrain, 0, n)
	for i := 0; i < n; i++ {
		c := dialLoadClient(t, ctx, wsURL)
		conns = append(conns, c)
		drains = append(drains, drainClient(c))
	}
	t.Cleanup(func() {
		for _, c := range conns {
			c.CloseNow()
		}
	})

	// 基线：客户端全部 attach 完成后双 GC 稳定读数。
	runtime.GC()
	time.Sleep(150 * time.Millisecond)
	runtime.GC()
	allocBase := readAlloc()
	smp := startLoadSamplers(base + "/metrics")

	start := time.Now()
	if err := conns[0].Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
		t.Fatalf("write 触发 INPUT: %v", err)
	}
	results := make([]loadDrain, n)
	for i := range drains {
		results[i] = awaitDrain(t, drains[i], 240*time.Second, fmt.Sprintf("client %d", i))
	}
	dur := time.Since(start)
	allocPeak, outboxMax := smp.stop()

	for i, r := range results {
		assertClosed1000(t, r, fmt.Sprintf("client %d", i))
	}

	// 结束 GC 后回基线 ±50%（drain 统计只持尾部采样，测试侧无驻留大对象）。
	for _, c := range conns {
		c.CloseNow()
	}
	runtime.GC()
	time.Sleep(150 * time.Millisecond)
	runtime.GC()
	allocPost := readAlloc()

	const memCeiling = 64 * 1024 * 1024 // 4× 账面最坏（32×512KiB outbox ≈ 16MiB + 共享帧）
	if allocPeak > memCeiling {
		t.Fatalf("Alloc 峰值 = %d, want ≤ %d（4× 账面最坏 16MiB——内存上界证伪）", allocPeak, memCeiling)
	}
	if allocPost > allocBase*3/2 || allocPost < allocBase/2 {
		t.Fatalf("GC 后 Alloc = %d, want 回基线 %d ±50%%（defunct/驻留泄漏信号）", allocPost, allocBase)
	}

	body := getMetrics(t, base+"/metrics")
	kicks, _ := metricSample(t, body, "wesh_clients_kicked_total")
	gateT, _ := metricSample(t, body, "wesh_credit_gate_transitions_total")
	memAlloc, _ := metricSample(t, body, "wesh_mem_alloc_bytes")
	if kicks != 0 {
		t.Fatalf("wesh_clients_kicked_total = %d, want 精确 0", kicks)
	}
	t.Logf("LOADDATA cell=memory_bound clients=%d profile=seq_flood(last=%d) slowlink=none kicks=%d gate_transitions=%d outbox_max=%d alloc_peak=%d alloc_base=%d alloc_post_gc=%d mem_alloc_end=%d dur_ms=%d",
		n, last, kicks, gateT, outboxMax, allocPeak, allocBase, allocPost, memAlloc, dur.Milliseconds())
}

// TestLoadGateTransitions（D-12 断言三）：突发流 × 限速读者格——产出 64KiB/30ms
// ≈ 2.2MB/s 突发 × 150 拍 ≈ 9.6MiB，单 rw 端 drain 限速 600KB/s。产出持续
// 超 drain → loopback 吸收带（~10MiB）填满后 outbox 承压 → 单体 rw 无离群
// 可踢（分工表：无其他未 blocked 可写端）→ 信用门闭合/重开周期循环。
// 断言：gateTransitions ≥ 2（门确实完成至少一次闭→开周期——机制实证）且
// 速率 ≤ 10 次/s（不震颤——50% 半水位迟滞下单周期至少搬运 256KiB，600KB/s
// 读者周期 ≥0.43s，10/s 为两个数量级裕度的病态震荡判界线）且 kicks==0
//（信用保护形态——合法慢端由门承载而非踢出）。计数/格时经 LOADDATA 上报。
func TestLoadGateTransitions(t *testing.T) {
	// 尾闸 sleep 1 同 gatedFloodArgv 注释纪律。
	argv := []string{"bash", "-c", `i=0; while [ $i -lt 150 ]; do head -c 65536 /dev/zero | tr '\0' 'A'; i=$((i+1)); sleep 0.03; done; sleep 1`}
	_, wsURL, _ := startTrackedServerWith(t, argv, server.Options{Writable: true})
	base := httpBaseOf(wsURL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	slow := dialLoadClient(t, ctx, wsURL)
	t.Cleanup(func() { slow.CloseNow() })
	drain := drainRateLimited(slow, 600*1024)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	allocBase := readAlloc()
	smp := startLoadSamplers(base + "/metrics")
	start := time.Now()

	r := awaitDrain(t, drain, 120*time.Second, "rate-limited reader (600KB/s)")
	dur := time.Since(start)
	allocPeak, outboxMax := smp.stop()

	assertClosed1000(t, r, "rate-limited reader (600KB/s)")

	body := getMetrics(t, base+"/metrics")
	kicks, _ := metricSample(t, body, "wesh_clients_kicked_total")
	gateT, _ := metricSample(t, body, "wesh_credit_gate_transitions_total")
	if kicks != 0 {
		t.Fatalf("wesh_clients_kicked_total = %d, want 精确 0（单体 rw 信用保护不踢）", kicks)
	}
	if gateT < 2 {
		t.Fatalf("wesh_credit_gate_transitions_total = %d, want ≥ 2（门至少完成一次闭→开周期——突发×限速格机制实证）", gateT)
	}
	if rate := float64(gateT) / dur.Seconds(); rate > 10 {
		t.Fatalf("信用门转换速率 = %.2f/s（%d 次 / %v）, want ≤ 10/s（连续震荡即证伪信号——50%% 半水位迟滞失效）", rate, gateT, dur)
	}
	t.Logf("LOADDATA cell=gate_transitions clients=1 profile=burst_64KiB/30ms(2.2MBps) slowlink=rate_limited_600KBps kicks=%d gate_transitions=%d outbox_max=%d alloc_peak=%d alloc_base=%d bytes=%d dur_ms=%d",
		kicks, gateT, outboxMax, allocPeak, allocBase, r.bytes, dur.Milliseconds())
}

// ====== 高频建销 defunct 三面 ======

// countFds 读 /proc/self/fd 目录项计数（Linux-only 夹具——调用点已 GOOS 门控）。
func countFds(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("read /proc/self/fd: %v", err)
	}
	return len(ents)
}

// readProcState 读 /proc/<pid>/stat 的 state 字段；进程已收割消失（健康归宿）
// 或形态异常返回空串。comm 字段可含空格与括号——取最后一个 ')' 之后的首字段
//（proc(5) stat 标准解析法）。
func readProcState(pid int) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "" // 已收割消失 = 健康归宿
	}
	s := string(b)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return ""
	}
	fields := strings.Fields(s[i+1:])
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// TestLoadDefunct（高频建销三面，Linux-only——/proc 口径，darwin t.Skip；load
// 测试本机手动跑 CI 不进）：基线 NumGoroutine + /proc/self/fd 计数 → N 轮
//（spawn Server + argv=["true"] 立即退出子进程 + 等待 exitf 收口 + killServer
// 轮内显式清理——非 t.Cleanup：fd/goroutine 回基线口径要求轮间释放，
// t.Cleanup 会把 200 个 listener 积压到测试尾）→ 终态 NumGoroutine/fd 回基线
// +容差 + 全部曾存子进程 /proc/<pid>/stat 无 Z 态（pty.Session Wait 唯一
// 收割者承诺的负载形态证据）。
//
// 容差 +4 论证：exitf 在 lifecycle goroutine 内被调（terminate 单点），收码即
// 该 goroutine 返回边缘；ReadLoop/inputWriter 的终结先于 exitf（Drain 关
// master → close(inputDone) 均排在 EXIT 广播之前），http.Serve goroutine 随
// ln.Close 即退——500ms settle + +4 裕度对时序 straggler 充分。
func TestLoadDefunct(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("defunct 三面为 Linux-only（/proc 口径）——darwin 分支 skip")
	}
	const rounds = 200

	// 基线采集前 settle：同进程内前序测试族的收尾 goroutine/http keep-alive 归位。
	runtime.GC()
	time.Sleep(500 * time.Millisecond)
	baseGor := runtime.NumGoroutine()
	baseFDs := countFds(t)

	pids := make([]int, 0, rounds)
	start := time.Now()
	for i := 0; i < rounds; i++ {
		sess, err := pty.Start([]string{"true"}, pty.StartOptions{Uid: -1, Gid: -1})
		if err != nil {
			t.Fatalf("round %d pty.Start: %v", i, err)
		}
		exitCh := make(chan int, 1)
		srv := server.New(sess, func(code int) { exitCh <- code }, server.Options{})
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("round %d net.Listen: %v", i, err)
		}
		go http.Serve(ln, srv.Handler())
		pids = append(pids, sess.Cmd.Process.Pid)

		select {
		case code := <-exitCh:
			if code != 0 {
				t.Fatalf("round %d exit code = %d, want 0（true 立即退出）", i, code)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("round %d exitf 5s 未触发（高频建销收口失败——lifecycle 卡死信号）", i)
		}
		killServer(ln, sess) // 轮内显式收口（每格独立收口纪律的 defunct 形态）
	}
	dur := time.Since(start)

	runtime.GC()
	time.Sleep(500 * time.Millisecond)
	endGor := runtime.NumGoroutine()
	endFDs := countFds(t)
	if endGor > baseGor+4 {
		t.Fatalf("goroutine 终态 = %d, want ≤ 基线 %d +4（%d 轮建销后 goroutine 泄漏——defunct 面一）", endGor, baseGor, rounds)
	}
	if endFDs > baseFDs+4 {
		t.Fatalf("fd 终态 = %d, want ≤ 基线 %d +4（%d 轮建销后 fd 泄漏——defunct 面二）", endFDs, baseFDs, rounds)
	}
	zombies := 0
	for _, pid := range pids {
		if readProcState(pid) == "Z" {
			zombies++
		}
	}
	if zombies > 0 {
		t.Fatalf("%d/%d 个曾存子进程残留 Z 态（收割者承诺失效——defunct 面三）", zombies, rounds)
	}
	t.Logf("LOADDATA cell=defunct rounds=%d goroutine_base=%d goroutine_end=%d fd_base=%d fd_end=%d zombies=%d dur_ms=%d",
		rounds, baseGor, endGor, baseFDs, endFDs, zombies, dur.Milliseconds())
}
