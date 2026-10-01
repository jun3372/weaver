package main

// 框架压测:持续高并发负载下的吞吐、延迟分位数与稳定性。
// 通过 -stress 显式开启,避免混入常规 go test:
//
//	go test ./examples/http/ -stress -run Stress -count=1
//	go test ./examples/http/ -stress -stress.duration=30s -stress.concurrency=128 -run Stress

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	stressFlag    = flag.Bool("stress", false, "运行压测用例")
	stressDur     = flag.Duration("stress.duration", 5*time.Second, "单场景压测时长")
	stressConc    = flag.Int("stress.concurrency", 64, "压测并发数")
	stressReload  = flag.Bool("stress.reload", true, "HTTP 压测期间叠加配置热更新")
	stressVerbose = flag.Bool("stress.verbose", false, "输出延迟分布明细")
)

// latencyStats 收集纳秒级延迟并输出分位数。
type latencyStats struct {
	mu sync.Mutex
	ns []int64
}

func (s *latencyStats) add(d time.Duration) {
	s.mu.Lock()
	s.ns = append(s.ns, int64(d))
	s.mu.Unlock()
}

func (s *latencyStats) report() (p50, p95, p99, max time.Duration, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n = len(s.ns)
	if n == 0 {
		return
	}
	sort.Slice(s.ns, func(i, j int) bool { return s.ns[i] < s.ns[j] })
	pick := func(q float64) time.Duration {
		idx := int(math.Ceil(q*float64(n-1))) + 0
		if idx >= n {
			idx = n - 1
		}
		return time.Duration(s.ns[idx])
	}
	if *stressVerbose {
		for _, q := range []float64{0.5, 0.9, 0.99, 0.999} {
			fmt.Printf("        p%v: %v\n", q*1000, pick(q))
		}
	}
	return pick(0.50), pick(0.95), pick(0.99), time.Duration(s.ns[n-1]), n
}

type stressResult struct {
	name   string
	ops    int64
	errs   int64
	p50    time.Duration
	p95    time.Duration
	p99    time.Duration
	max    time.Duration
	rps    float64
	gorHi  int
	gorEnd int
}

func (r stressResult) log(t *testing.T) {
	t.Helper()
	t.Logf("%-28s 吞吐 %10.0f ops/s  p50 %8v  p95 %8v  p99 %8v  max %8v  错误 %d  goroutine %d→%d",
		r.name, r.rps, r.p50, r.p95, r.p99, r.max, r.errs, r.gorHi, r.gorEnd)
}

// runLoad 以 concurrency 个 worker 跑 dur 时长,op 返回错误计为失败。
func runLoad(t *testing.T, name string, op func() error) stressResult {
	t.Helper()
	var (
		ops, errs atomic.Int64
		stats     latencyStats
		stop      = make(chan struct{})
		wg        sync.WaitGroup
	)
	gorHi := runtime.NumGoroutine()

	for i := 0; i < *stressConc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				start := time.Now()
				if err := op(); err != nil {
					errs.Add(1)
					continue
				}
				stats.add(time.Since(start))
				ops.Add(1)
			}
		}()
	}

	time.Sleep(*stressDur)
	close(stop)
	wg.Wait()

	p50, p95, p99, max, n := stats.report()
	r := stressResult{
		name: name, ops: int64(n), errs: errs.Load(),
		p50: p50, p95: p95, p99: p99, max: max,
		rps:   float64(n) / stressSec(),
		gorHi: gorHi, gorEnd: runtime.NumGoroutine(),
	}
	if r.errs > 0 {
		t.Errorf("%s 出现 %d 次错误", name, r.errs)
	}
	return r
}

func stressSec() float64 { return stressDur.Seconds() }

// --- 场景 1: HTTP 持续高并发,可选叠加配置热更新 ---

func TestStressHTTP(t *testing.T) {
	if !*stressFlag {
		t.Skip("需要 -stress 开启")
	}

	valid := map[string]bool{"stress-A\n": true, "stress-B\n": true}
	var reloadStop chan struct{}
	var reloadDone sync.WaitGroup
	if *stressReload {
		// 从 TestMain 保存的临时目录定位配置文件
		conf := os.Getenv("SERVICE_CONFIG")
		reloadStop = make(chan struct{})
		reloadDone.Add(1)
		go func() {
			defer reloadDone.Done()
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			values := []string{"stress-A", "stress-B"}
			// 立即对齐首个合法值,之后原地写入(真实编辑场景),fsnotify 触发热更新
			body := func(v string) []byte {
				return []byte(fmt.Sprintf("app:\n  Name: bench\nhttp:\n  Addr: \":0\"\n  Message: %q\ntcp:\n  Addr: \":0\"\nudp:\n  Addr: \":0\"\nweaver:\n  Logger:\n    Level: \"error\"\n    Type: \"json\"\n", v))
			}
			_ = os.WriteFile(conf, body(values[0]), 0o600)
			for i := 1; ; i++ {
				select {
				case <-reloadStop:
					return
				case <-ticker.C:
					_ = os.WriteFile(conf, body(values[i%2]), 0o600)
				}
			}
		}()
	}

	client := &http.Client{Transport: &http.Transport{
		MaxIdleConns:        *stressConc,
		MaxIdleConnsPerHost: *stressConc,
	}}
	res := runLoad(t, "HTTP 高并发", func() error {
		resp, err := client.Get("http://" + httpAddr + "/")
		if err != nil {
			return err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if !valid[string(body)] {
			return fmt.Errorf("非法响应 %q", body)
		}
		return nil
	})
	res.log(t)

	if reloadStop != nil {
		close(reloadStop)
		reloadDone.Wait()
	}
}

// --- 场景 2: TCP 大量并发连接 + 回显往返 ---

func TestStressTCP(t *testing.T) {
	if !*stressFlag {
		t.Skip("需要 -stress 开启")
	}

	var (
		ops, errs atomic.Int64
		stats     latencyStats
		wg        sync.WaitGroup
		stop      = make(chan struct{})
	)
	gorHi := runtime.NumGoroutine()

	// 每个连接一个 worker:连接数 = 并发数,模拟长连接高频往返
	for i := 0; i < *stressConc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", tcpAddr)
			if err != nil {
				errs.Add(1)
				return
			}
			defer conn.Close()
			msg := []byte("stress\n")
			buf := make([]byte, len(msg))
			for {
				select {
				case <-stop:
					return
				default:
				}
				start := time.Now()
				if _, err := conn.Write(msg); err != nil {
					errs.Add(1)
					return
				}
				// 短读超时:停止信号发出后最多 1s 内解除阻塞退出
				_ = conn.SetReadDeadline(time.Now().Add(time.Second))
				if _, err := io.ReadFull(conn, buf); err != nil {
					select {
					case <-stop:
						return
					default:
					}
					errs.Add(1)
					return
				}
				if !bytes.Equal(buf, msg) {
					errs.Add(1)
					return
				}
				stats.add(time.Since(start))
				ops.Add(1)
			}
		}()
	}

	time.Sleep(*stressDur)
	close(stop)
	wg.Wait()

	p50, p95, p99, max, n := stats.report()
	res := stressResult{
		name: "TCP 并发连接回显", ops: int64(n), errs: errs.Load(),
		p50: p50, p95: p95, p99: p99, max: max,
		rps:   float64(n) / stressSec(),
		gorHi: gorHi, gorEnd: runtime.NumGoroutine(),
	}
	res.log(t)
	if res.errs > 0 {
		t.Errorf("TCP 压测出现 %d 次错误", res.errs)
	}
}

// --- 场景 3: UDP 报文风暴 ---

func TestStressUDP(t *testing.T) {
	if !*stressFlag {
		t.Skip("需要 -stress 开启")
	}

	res := runLoad(t, "UDP 报文回显", func() error {
		conn, err := net.Dial("udp", udpAddr)
		if err != nil {
			return err
		}
		defer conn.Close()
		if _, err := conn.Write([]byte("stress")); err != nil {
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 16)
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		// 示例 handler 以 ToUpper 回显
		if string(buf[:n]) != "STRESS" {
			return fmt.Errorf("非法回显 %q", buf[:n])
		}
		return nil
	})
	res.log(t)
}

// --- 场景 4: 压测后 goroutine 稳定性检查 ---

func TestStressGoroutineStability(t *testing.T) {
	if !*stressFlag {
		t.Skip("需要 -stress 开启")
	}

	before := runtime.NumGoroutine()
	// 短促高并发连接:大量连接快速建立并关闭,检查无 goroutine 泄漏
	var wg sync.WaitGroup
	for i := 0; i < *stressConc*4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", tcpAddr)
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("leak\n"))
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			buf := make([]byte, 8)
			_, _ = conn.Read(buf)
			_ = conn.Close()
		}()
	}
	wg.Wait()

	// 等待服务端回收连接 goroutine
	time.Sleep(2 * time.Second)
	after := runtime.NumGoroutine()
	t.Logf("goroutine: 压测场景前 %d,短连接风暴后 %d", before, after)
	if after > before+*stressConc {
		t.Errorf("goroutine 疑似泄漏: %d → %d", before, after)
	}
}
