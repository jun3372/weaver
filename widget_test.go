package weaver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/jun3372/weaver/internal/reflection"
	"github.com/jun3372/weaver/runtime/codegen"
)

type panicIntf interface {
	Ping() string
}

type panicErrorComp struct {
	Implements[panicIntf]
}

func (c *panicErrorComp) Start(context.Context) error {
	panic(errors.New("boom-error"))
}

func (c *panicErrorComp) Ping() string { return "pong" }

type panicStringComp struct {
	Implements[panicIntf]
}

func (c *panicStringComp) Start(context.Context) error {
	panic("boom-string")
}

func (c *panicStringComp) Ping() string { return "pong" }

type errorStartComp struct {
	Implements[panicIntf]
}

func (c *errorStartComp) Start(context.Context) error {
	return errors.New("start-failed")
}

func (c *errorStartComp) Ping() string { return "pong" }

type blockingIntf interface {
	Ping() string
}

type blockingComp struct {
	Implements[blockingIntf]
}

// Start 模拟长驻服务：阻塞直到 ctx 结束。
func (c *blockingComp) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (c *blockingComp) Ping() string { return "pong" }

func newTestWidget(t *testing.T, regs ...*codegen.Registration) (context.Context, *widget) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return ctx, newWidget(ctx, cancel, nil, regs)
}

func awaitCancel(t *testing.T, ctx context.Context, what string) {
	t.Helper()

	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatalf("%s 后应触发 context 取消，3 秒内未发生", what)
	}
}

func TestStartCancelsContextWhenStartPanicsWithError(t *testing.T) {
	ctx, w := newTestWidget(t, &codegen.Registration{
		Name:      "test/panicError",
		Interface: reflection.Type[panicIntf](),
		Impl:      reflect.TypeFor[panicErrorComp](),
	})

	if _, err := w.getImpl(reflect.TypeFor[panicErrorComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}

	w.start(context.Background())
	awaitCancel(t, ctx, "组件 Start panic（error 值）")
}

func TestStartCancelsContextWhenStartPanicsWithNonError(t *testing.T) {
	ctx, w := newTestWidget(t, &codegen.Registration{
		Name:      "test/panicString",
		Interface: reflection.Type[panicIntf](),
		Impl:      reflect.TypeFor[panicStringComp](),
	})

	if _, err := w.getImpl(reflect.TypeFor[panicStringComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}

	w.start(context.Background())
	awaitCancel(t, ctx, "组件 Start panic（非 error 值）")
}

func TestStartCancelsContextWhenStartReturnsError(t *testing.T) {
	ctx, w := newTestWidget(t, &codegen.Registration{
		Name:      "test/errorStart",
		Interface: reflection.Type[panicIntf](),
		Impl:      reflect.TypeFor[errorStartComp](),
	})

	if _, err := w.getImpl(reflect.TypeFor[errorStartComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}

	w.start(context.Background())
	awaitCancel(t, ctx, "组件 Start 返回错误")
}

// start 必须异步启动组件：长驻阻塞型 Start（如监听服务）不能阻塞 start 返回，
// 否则 Run 会永远无法进入 app 函数（examples/hello 的 chat 组件即此形态）。
func TestStartDoesNotBlockOnLongRunningComponents(t *testing.T) {
	_, w := newTestWidget(t, &codegen.Registration{
		Name:      "test/blocking",
		Interface: reflection.Type[blockingIntf](),
		Impl:      reflect.TypeFor[blockingComp](),
	})

	if _, err := w.getImpl(reflect.TypeFor[blockingComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}

	done := make(chan struct{})
	go func() {
		w.start(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("start 被阻塞型组件 Start 阻塞，应在启动后立即返回")
	}
}

type reloadIntf interface {
	Ping() string
}

type reloadComp struct {
	Implements[reloadIntf]
	WithConfig[struct{ Name string }] `conf:"reload"`
}

func (c *reloadComp) Start(context.Context) error { return nil }
func (c *reloadComp) Ping() string                { return "pong" }

type lateIntf interface {
	Ping() string
}

type lateComp struct {
	Implements[lateIntf]
	WithConfig[struct{ Name string }] `conf:"late"`
}

func (c *lateComp) Start(context.Context) error { return nil }
func (c *lateComp) Ping() string                { return "pong" }

func newTestWidgetWithConf(t *testing.T, regs ...*codegen.Registration) *widget {
	t.Helper()

	confFile := filepath.Join(t.TempDir(), "weaver.yaml")
	if err := os.WriteFile(confFile, []byte("weaver:\n  logger:\n    level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	conf := viper.New()
	conf.SetConfigFile(confFile)
	if err := conf.ReadInConfig(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return newWidget(ctx, cancel, conf, regs)
}

// 组件创建（写 components/watchConfig）与配置热更新（读两者）并发时不应产生数据竞争。
// 需以 go test -race 运行才能暴露问题。
func TestConfigReloadConcurrentWithoutDataRace(t *testing.T) {
	w := newTestWidgetWithConf(t,
		&codegen.Registration{
			Name:      "test/reload",
			Interface: reflection.Type[reloadIntf](),
			Impl:      reflect.TypeFor[reloadComp](),
		},
		&codegen.Registration{
			Name:      "test/late",
			Interface: reflection.Type[lateIntf](),
			Impl:      reflect.TypeFor[lateComp](),
		},
	)

	// 预创建第一个组件，使 watchConfig 中已有回调
	if _, err := w.getImpl(reflect.TypeFor[reloadComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				w.reloadConfig()
			}
		}
	})

	// 热更新进行中创建新组件：components 写 + watchConfig append
	time.Sleep(50 * time.Millisecond) // 确保热更新循环已在运行
	if _, err := w.getImpl(reflect.TypeFor[lateComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	close(stop)
	wg.Wait()
}
