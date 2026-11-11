package weaver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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

	return ctx, newWidget(ctx, cancel, nil, regs, "WEAVER")
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

// --- shutdown 依赖拓扑排序 ---

type topoLeafIntf interface {
	Ping() string
}

type topoLeafComp struct {
	Implements[topoLeafIntf]
}

func (c *topoLeafComp) Shutdown(context.Context) error { recordShutdown("test/leaf"); return nil }
func (c *topoLeafComp) Ping() string                   { return "pong" }

type topoMidIntf interface {
	Ping() string
}

type topoMidComp struct {
	Implements[topoMidIntf]
	Ref[topoLeafIntf]
}

func (c *topoMidComp) Shutdown(context.Context) error { recordShutdown("test/mid"); return nil }
func (c *topoMidComp) Ping() string                   { return "pong" }

type topoTopIntf interface {
	Ping() string
}

type topoTopComp struct {
	Implements[topoTopIntf]
	Ref[topoMidIntf]
}

func (c *topoTopComp) Shutdown(context.Context) error { recordShutdown("test/top"); return nil }
func (c *topoTopComp) Ping() string                   { return "pong" }

var (
	shutdownMu    sync.Mutex
	shutdownOrder []string
)

func recordShutdown(name string) {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	shutdownOrder = append(shutdownOrder, name)
}

// 被依赖的组件必须最后关闭：top 依赖 mid、mid 依赖 leaf，
// 关闭顺序应为 top → mid → leaf。
func TestShutdownFollowsReverseDependencyOrder(t *testing.T) {
	shutdownMu.Lock()
	shutdownOrder = nil
	shutdownMu.Unlock()

	_, w := newTestWidget(t,
		&codegen.Registration{
			Name:      "test/leaf",
			Interface: reflection.Type[topoLeafIntf](),
			Impl:      reflect.TypeFor[topoLeafComp](),
		},
		&codegen.Registration{
			Name:      "test/mid",
			Interface: reflection.Type[topoMidIntf](),
			Impl:      reflect.TypeFor[topoMidComp](),
		},
		&codegen.Registration{
			Name:      "test/top",
			Interface: reflection.Type[topoTopIntf](),
			Impl:      reflect.TypeFor[topoTopComp](),
		},
	)

	// 拉起 top，经 Ref 注入递归实例化 mid 与 leaf 并记下依赖边
	if _, err := w.getImpl(reflect.TypeFor[topoTopComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}

	w.shutdown(context.Background())

	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	want := []string{"test/top", "test/mid", "test/leaf"}
	if !slices.Equal(shutdownOrder, want) {
		t.Fatalf("shutdown 顺序错误: got %v, want %v", shutdownOrder, want)
	}
}

// --- 配置 Validate() 校验钩子 ---

type validatedConf struct {
	Port int
}

func (c validatedConf) Validate() error {
	if c.Port <= 0 {
		return errors.New("port 必须为正数")
	}
	return nil
}

type validatedIntf interface {
	Ping() string
}

type validatedComp struct {
	Implements[validatedIntf]
	WithConfig[validatedConf] `conf:"validated"`
}

func (c *validatedComp) Ping() string { return "pong" }

func newValidatedWidget(t *testing.T, port int) (*widget, *validatedComp) {
	t.Helper()

	confFile := filepath.Join(t.TempDir(), "weaver.yaml")
	content := fmt.Sprintf("validated:\n  port: %d\n", port)
	if err := os.WriteFile(confFile, []byte(content), 0o644); err != nil {
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
		Name:      "test/validated",
		Interface: reflection.Type[validatedIntf](),
		Impl:      reflect.TypeFor[validatedComp](),
	}}, "WEAVER")

	c, err := w.getImpl(reflect.TypeFor[validatedComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	return w, c.(*validatedComp)
}

func TestValidateAcceptsConfig(t *testing.T) {
	_, c := newValidatedWidget(t, 8080)
	if got := c.Config().Port; got != 8080 {
		t.Fatalf("合法配置应注入成功: got port %d, want 8080", got)
	}
}

// pointerValidatedConf 用指针接收者实现 Validate,框架同样应识别并执行校验。
type pointerValidatedConf struct {
	Port int
}

func (c *pointerValidatedConf) Validate() error {
	if c.Port <= 0 {
		return errors.New("port 必须为正数")
	}
	return nil
}

type pointerValidatedComp struct {
	Implements[validatedIntf]
	WithConfig[pointerValidatedConf] `conf:"pvalidated"`
}

func (c *pointerValidatedComp) Ping() string { return "pong" }

func TestValidateSupportsPointerReceiver(t *testing.T) {
	confFile := filepath.Join(t.TempDir(), "weaver.yaml")
	if err := os.WriteFile(confFile, []byte("pvalidated:\n  port: -1\n"), 0o644); err != nil {
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
		Name:      "test/pvalidated",
		Interface: reflection.Type[validatedIntf](),
		Impl:      reflect.TypeFor[pointerValidatedComp](),
	}}, "WEAVER")

	c, err := w.getImpl(reflect.TypeFor[pointerValidatedComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	if got := c.(*pointerValidatedComp).Config().Port; got != 0 {
		t.Fatalf("指针接收者的 Validate 也应生效并拒绝非法配置: got port %d, want 0", got)
	}
}

func TestValidateRejectsInitialConfig(t *testing.T) {
	_, c := newValidatedWidget(t, -1)
	if got := c.Config().Port; got != 0 {
		t.Fatalf("非法配置应被拒绝并保留零值: got port %d, want 0", got)
	}
}

func TestValidateRejectsHotReloadKeepsOldValue(t *testing.T) {
	w, c := newValidatedWidget(t, 8080)

	w.conf.Set("validated.port", -1)
	w.reloadConfig()

	if got := c.Config().Port; got != 8080 {
		t.Fatalf("热更新非法值应被拒绝并保留旧值: got port %d, want 8080", got)
	}
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

	return newWidget(ctx, cancel, conf, regs, "WEAVER")
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

// --- Listener/WithConfig 反射识别失败的显式诊断 ---

// 命名类型包装(非别名):Type.Name() 不含 "Listener[" / "WithConfig[" 前缀,
// 框架会静默跳过,装配期应输出 Warn 告警。
type renamedListener Listener[Handler]
type renamedWithConfig WithConfig[struct{ Word string }]

type diagIntf interface{ Ping() string }
type chatListenerIntf interface{ Ping() string } // 接口名含 "Listener" 字样,用于误报验证

type diagComp struct {
	Implements[diagIntf]
	renamedL renamedListener   `conf:"renamed"`
	renamedC renamedWithConfig `conf:"renamedcfg"`
	netL     net.Listener      `conf:"netln"` // 外部同名类型
	peer     Ref[diagPeerIntf] // Ref 字段,接口名不含敏感词,装配需能解析到已注册组件
}

// Ref 误报验证:接口名含 "Listener" 字样的组件原语字段不得触发告警。
type diagRefComp struct {
	Implements[diagIntf]
	normal Ref[chatListenerIntf]
}

func (c *diagComp) Ping() string { return "pong" }

type diagPeerIntf interface{ Ping() string }

// diagPeerComp 是同进程正常组件,告警组件的装配不得影响它。
type diagPeerComp struct {
	Implements[diagPeerIntf]
	WithConfig[struct{ Word string }] `conf:"diagpeer"`
}

func (c *diagPeerComp) Ping() string { return "pong" }

// newDiagWidget 构造装配日志写入 buf 的 widget。
func newDiagWidget(t *testing.T, buf *bytes.Buffer, regs ...*codegen.Registration) *widget {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	w := newWidget(ctx, cancel, viper.New(), regs, "WEAVER")
	// 同包替换装配日志输出,避免依赖 slog 全局默认 logger(会被其它测试改写)
	w.log = slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return w
}

func TestWarnsOnUnrecognizedListenerAndConfigFields(t *testing.T) {
	var buf bytes.Buffer
	w := newDiagWidget(t, &buf,
		&codegen.Registration{
			Name:      "test/diag",
			Interface: reflection.Type[diagIntf](),
			Impl:      reflect.TypeFor[diagComp](),
		},
		&codegen.Registration{
			Name:      "test/diagpeer",
			Interface: reflection.Type[diagPeerIntf](),
			Impl:      reflect.TypeFor[diagPeerComp](),
		},
		&codegen.Registration{
			Name:      "test/chatlistener",
			Interface: reflection.Type[chatListenerIntf](),
			Impl:      reflect.TypeFor[diagPeerComp](),
		},
	)

	// 告警组件装配:不得 panic,组件仍正常创建(只是服务/配置不生效)
	c, err := w.getImpl(reflect.TypeFor[diagComp]())
	if err != nil {
		t.Fatalf("带异常字段的组件应仍能装配成功: %v", err)
	}
	if c.(*diagComp).Ping() != "pong" {
		t.Fatal("告警组件应仍可正常调用")
	}

	// 同进程其它组件装配不受影响(含被 diagComp.Ref 引用的 diagPeerComp)
	if _, err := w.getImpl(reflect.TypeFor[diagPeerComp]()); err != nil {
		t.Fatalf("同进程其它组件装配不应受影响: %v", err)
	}

	out := buf.String()
	if n := strings.Count(out, "未被框架识别为服务字段"); n != 1 {
		t.Fatalf("Listener 类告警应恰好输出一次(同一组件同类合并), got %d 次:\n%s", n, out)
	}
	if n := strings.Count(out, "未被框架识别为配置托管字段"); n != 1 {
		t.Fatalf("WithConfig 类告警应恰好输出一次, got %d 次:\n%s", n, out)
	}
	for _, want := range []string{"renamedL", "renamedC", "netL", "diagComp"} {
		if !strings.Contains(out, want) {
			t.Fatalf("告警应包含 %q, got:\n%s", want, out)
		}
	}

	// 常见组件原语不得误报:normal 字段是 Ref[chatListenerIntf],接口名含 "Listener" 字样
	var refBuf bytes.Buffer
	w2 := newDiagWidget(t, &refBuf,
		&codegen.Registration{
			Name:      "test/diagref",
			Interface: reflection.Type[diagIntf](),
			Impl:      reflect.TypeFor[diagRefComp](),
		},
		&codegen.Registration{
			Name:      "test/chatlistener",
			Interface: reflection.Type[chatListenerIntf](),
			Impl:      reflect.TypeFor[diagPeerComp](),
		},
	)
	if _, err := w2.getImpl(reflect.TypeFor[diagRefComp]()); err != nil {
		t.Fatalf("Ref 字段组件装配不应失败: %v", err)
	}
	if refOut := refBuf.String(); strings.Contains(refOut, "未被框架识别") {
		t.Fatalf("Ref[接口名含 Listener 字样] 字段不应触发告警, got:\n%s", refOut)
	}
}

// 正常 weaver.Listener[H] / weaver.WithConfig[T] 场景不得产生识别类告警;
// 纯类型别名(=)的反射名与原类型一致,识别与功能均不受影响,同样不应告警。
// 注:Listener/WithConfig 字段须匿名内嵌 —— 命名类型别名作具名小写字段属未导出
// 字段,SetConfig 反射调用会 panic,那是另一回事,不在本诊断覆盖范围内。
type AliasListener = Listener[Handler]

type diagOKComp struct {
	Implements[diagIntf]
	AliasListener               `conf:"okalias"` // 纯类型别名,应被正常识别
	WithConfig[struct{ A int }] `conf:"okcfg"`
}

func (c *diagOKComp) Ping() string { return "pong" }

func TestNoWarnOnRecognizedListenerAndConfigFields(t *testing.T) {
	var buf bytes.Buffer
	w := newDiagWidget(t, &buf,
		&codegen.Registration{
			Name:      "test/diagok",
			Interface: reflection.Type[diagIntf](),
			Impl:      reflect.TypeFor[diagOKComp](),
		},
		&codegen.Registration{
			Name:      "test/diagpeer",
			Interface: reflection.Type[diagPeerIntf](),
			Impl:      reflect.TypeFor[diagPeerComp](),
		},
	)

	if _, err := w.getImpl(reflect.TypeFor[diagOKComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}
	if _, err := w.getImpl(reflect.TypeFor[diagPeerComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}

	if out := buf.String(); strings.Contains(out, "未被框架识别") {
		t.Fatalf("正常 Listener/WithConfig 与纯类型别名字段不应产生识别类告警, got:\n%s", out)
	}
}
