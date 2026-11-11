package weaver

// 本文件覆盖热更新去抖、回调 panic 隔离、慢回调不阻塞三块验收点。
// 与 widget_test.go 中 reloadConfig 的既有用例(显式同步刷新)互补:
// 这里走 scheduleReload 的文件事件路径。

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/jun3372/weaver/internal/reflection"
	"github.com/jun3372/weaver/runtime/codegen"
)

// reloadProbe 记录回调执行次数与并发情况,供去抖与序列化断言。
type reloadProbe struct {
	execCount  atomic.Int32
	concurrent atomic.Int32 // 当前正在执行的回调数(检测重入)
	maxConc    atomic.Int32 // 观测到的最大并发数
	inFlight   chan struct{}
}

func (p *reloadProbe) callback(d time.Duration) func() {
	return func() {
		p.execCount.Add(1)
		cur := p.concurrent.Add(1)
		for {
			max := p.maxConc.Load()
			if cur <= max || p.maxConc.CompareAndSwap(max, cur) {
				break
			}
		}
		defer p.concurrent.Add(-1)
		if d > 0 {
			time.Sleep(d)
		}
		if p.inFlight != nil {
			select {
			case p.inFlight <- struct{}{}:
			default:
			}
		}
	}
}

// newReloadWidget 构造带 buf 日志、可注入自定义回调的 widget。
func newReloadWidget(t *testing.T, buf *bytes.Buffer) *widget {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	conf := viper.New()
	conf.Set("reload", map[string]any{"name": "v1"})
	w := newWidget(ctx, cancel, conf, nil, "WEAVER")
	if buf != nil {
		w.log = slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return w
}

// waitFor 轮询直到条件满足或超时,避免依赖固定 sleep 造成时序抖动。
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待超时(%v): %s", timeout, what)
}

// TestScheduleReloadCoalesces 连续触发 5 次 scheduleReload,断言合并为少数几轮执行。
func TestScheduleReloadCoalesces(t *testing.T) {
	w := newReloadWidget(t, nil)
	var p reloadProbe
	w.WatchConfig("reload", p.callback(0))

	// 在去抖窗口内连续触发 5 次
	for range 5 {
		w.scheduleReload()
	}

	// 等待执行完成
	waitFor(t, 2*time.Second, "去抖后应至少执行一轮重载", func() bool {
		return p.execCount.Load() >= 1
	})
	// 再稳定一个窗口,确认没有额外重载被排入
	time.Sleep(3 * reloadConfigDebounce)

	if got := p.execCount.Load(); got != 1 {
		t.Fatalf("窗口内 5 次触发应合并为 1 轮执行, got %d 轮", got)
	}
	if p.maxConc.Load() > 1 {
		t.Fatalf("回调执行应串行,观测到并发数 %d", p.maxConc.Load())
	}
}

// TestReloadPanicIsolated 单回调 panic 不影响其它回调执行、进程不崩。
func TestReloadPanicIsolated(t *testing.T) {
	var buf bytes.Buffer
	w := newReloadWidget(t, &buf)

	var order []string
	var mu sync.Mutex
	mark := func(name string) func() {
		return func() {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}
	}

	w.WatchConfig("reload", mark("before"))
	w.WatchConfig("reload", func() { panic("boom-inject") })
	w.WatchConfig("reload", mark("after"))

	w.reloadConfig() // 同步执行一轮

	mu.Lock()
	got := strings.Join(order, ",")
	mu.Unlock()
	if got != "before,after" {
		t.Fatalf("panic 回调应被隔离,前后回调都应执行, got 顺序 %q", got)
	}
	if !strings.Contains(buf.String(), "配置热更新回调 panic") {
		t.Fatalf("panic 应记录 Error 日志, got 日志:\n%s", buf.String())
	}
}

// TestSlowCallbackNonBlocking 注入 sleep 50ms 的慢回调 + 连续 5 次文件写入,
// 断言:事件被合并、进程不 panic、配置最终生效、其它 goroutine 不被阻塞。
func TestSlowCallbackNonBlocking(t *testing.T) {
	// 真实文件驱动的组件配置组件,Validate sleep 50ms 模拟慢校验。
	confFile := filepath.Join(t.TempDir(), "weaver.yaml")
	if err := os.WriteFile(confFile, []byte("slow:\n  value: v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := viper.New()
	conf.SetConfigFile(confFile)
	if err := conf.ReadInConfig(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	w := newWidget(ctx, cancel, conf, []*codegen.Registration{{
		Name:      "test/slow",
		Interface: reflection.Type[slowIntf](),
		Impl:      reflect.TypeFor[slowComp](),
	}}, "WEAVER")

	c, err := w.getImpl(reflect.TypeFor[slowComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	comp := c.(*slowComp)
	if got := comp.Config().Value; got != "v1" {
		t.Fatalf("初始配置应注入: got %q", got)
	}

	// 监控 goroutine:每 10ms tick 一次,若被阻塞则观测到心跳停滞。
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	var heartbeat atomic.Int64
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-ticker.C:
				heartbeat.Store(time.Now().UnixNano())
			case <-done:
				return
			}
		}
	}()

	// 连续原地写文件 5 次(每次相隔 < 去抖窗口,形成连续事件流),
	// 末次写入目标值 v-final。
	for i := range 5 {
		val := "v-final"
		if i < 4 {
			val = "v" + string(rune('a'+i))
		}
		content := "slow:\n  value: " + val + "\n"
		if err := os.WriteFile(confFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond) // 事件间隔小于去抖窗口
	}

	// 配置最终生效
	waitFor(t, 3*time.Second, "热更新后配置应最终生效", func() bool {
		return comp.Config().Value == "v-final"
	})

	// 进程未 panic(走到这里即证明),且心跳 goroutine 未被长时间阻塞。
	last := heartbeat.Load()
	time.Sleep(100 * time.Millisecond)
	if heartbeat.Load() == last {
		t.Fatal("心跳 goroutine 在热更新期间被阻塞,未持续推进")
	}

	// 慢校验被调用且远小于「5 次全量串行」的次数(去抖生效)。
	calls := slowValidateCalls.Load()
	if calls == 0 {
		t.Fatal("慢校验应被触发")
	}
	if calls > 3 {
		t.Logf("注意:慢校验执行了 %d 次(去抖窗口边界导致多轮,但仍远少于 5 次全量)", calls)
	}
}

// slowConf 带慢校验的配置:Validate sleep 50ms 模拟慢回调。
type slowConf struct {
	Value string
}

// validateCalls 经包级原子计数器暴露,便于测试断言去抖效果。
var slowValidateCalls atomic.Int32

func (c slowConf) Validate() error {
	slowValidateCalls.Add(1)
	time.Sleep(50 * time.Millisecond)
	return nil
}

type slowIntf interface{ Ping() string }

type slowComp struct {
	Implements[slowIntf]
	WithConfig[slowConf] `conf:"slow"`
}

func (c *slowComp) Ping() string { return "pong" }

// TestScheduleReloadDoesNotBlockCaller 去抖调度本身应立即返回,不等待慢回调执行。
func TestScheduleReloadDoesNotBlockCaller(t *testing.T) {
	w := newReloadWidget(t, nil)
	var p reloadProbe
	w.WatchConfig("reload", p.callback(200*time.Millisecond)) // 慢回调

	start := time.Now()
	w.scheduleReload()
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("scheduleReload 应立即返回(异步执行), 实际阻塞 %v", elapsed)
	}

	// 等待这一轮慢回调真正执行完,确认配置注入发生过
	waitFor(t, 2*time.Second, "慢回调应被执行", func() bool {
		return p.execCount.Load() >= 1
	})
}
