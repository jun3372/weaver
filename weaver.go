package weaver

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"

	"github.com/jun3372/weaver/internal/reflection"
	"github.com/jun3372/weaver/runtime/codegen"
	"github.com/jun3372/weaver/runtime/logger"
	"github.com/jun3372/weaver/version"
)

type Main any

var (
	flagsOnce sync.Once
	confFile  string
	showVer   bool
)

func parseFlags() {
	flagsOnce.Do(func() {
		flag.StringVar(&confFile, "conf", os.Getenv("SERVICE_CONFIG"), "config file path")
		flag.BoolVar(&showVer, "version", strings.ToLower(os.Getenv("SERVICE_VERSION")) == "true", "print version info")
	})
	// 宿主程序可能已自行调用过 flag.Parse
	if !flag.Parsed() {
		flag.Parse()
	}
}

func Run[T any, P PointerToMain[T]](ctx context.Context, app func(context.Context, *T) error) error {
	parseFlags()

	if showVer {
		version.PrintVersion()
		return nil
	}

	var conf *viper.Viper
	if confFile != "" {
		conf = viper.New()
		conf.SetConfigFile(confFile)
		if err := conf.ReadInConfig(); err != nil {
			return errors.Errorf("Fatal error config file: %v", err)
		}
	}

	var cancel context.CancelFunc
	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	widget := newWidget(ctx, cancel, conf, codegen.Registered())
	main, err := widget.getImpl(reflection.Type[T]())
	if err != nil {
		return err
	}

	// 启动组件。
	//
	// 错误契约：start 等待全部组件 Start 启动后即返回,不等待长驻 Start;
	// Start 同步快速失败时立刻中止 Run 并返回该错误,其余失败或 panic
	// 会调用 cancel() 使 ctx 结束并记录 ERROR 日志;app 必须监听 ctx
	// 否则进程会带着失败的组件继续运行。
	if err := widget.start(widget.ctx); err != nil {
		widget.shutdown(context.Background())
		return err
	}

	if m, ok := main.(*T); !ok {
		return errors.New("main type error")
	} else {
		err = app(ctx, m)
	}

	cancel()
	widget.shutdown(context.Background())
	return err
}

// RunComponent 启动全部组件后不执行额外主逻辑,阻塞等待退出信号
// (SIGINT/SIGQUIT/SIGTERM)或组件调用 Exec(),随后优雅关闭并返回 nil。
// app 仅用于类型推断,传 (*T)(nil) 即可:
//
//	if err := weaver.RunComponent(ctx, (*app)(nil)); err != nil {
//		panic(err)
//	}
func RunComponent[T any, P PointerToMain[T]](ctx context.Context, app *T) error {
	return Run[T, P](ctx, func(ctx context.Context, _ *T) error {
		<-ctx.Done()
		return ctx.Err()
	})
}

type WithConfig[T any] struct {
	mu     sync.RWMutex
	config T
}

// Config 返回配置的副本,配置热更新期间的读取是并发安全的。
func (c *WithConfig[T]) Config() T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// SetConfig 整体替换配置,由框架在配置注入/热更新时通过反射调用。
func (c *WithConfig[T]) SetConfig(v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = v
}

type Ref[T any] struct{ value T }

func (r Ref[T]) isRef() {}
func (r Ref[T]) Get() T { return r.value }
func (r *Ref[T]) setRef(value any) {
	r.value = value.(T)
}

type PointerToMain[T any] interface {
	*T
	InstanceOf[Main]
}

type InstanceOf[T any] interface {
	implements(_ T)
}
type Implements[T any] struct {
	// Component logger.
	logger *slog.Logger
	exec   context.CancelFunc

	// weaverInfo *weaver.WeaverInfo

	// Given a component implementation type, there is currently no nice way,
	// using reflection, to get the corresponding component interface type [1].
	// The component_interface_type field exists to make it possible.
	//
	// [1]: https://github.com/golang/go/issues/54393.
	//
	//lint:ignore U1000 See comment above.
	component_interface_type T

	// We embed implementsImpl so that component implementation structs
	// implement the Unrouted interface by default but implement the
	// RoutedBy[T] interface when they embed WithRouter[T].
	implementsImpl
}

type implementsImpl struct{}

func (i Implements[T]) Logger(ctx context.Context) *slog.Logger {
	l := i.logger
	s := trace.SpanContextFromContext(ctx)
	if s.HasTraceID() {
		l = l.With(logger.TRACE_ID_KEY, s.TraceID().String())
	}
	if s.HasSpanID() {
		l = l.With(logger.SPAN_ID_KEY, s.SpanID().String())
	}
	return l
}

func (i *Implements[T]) setLogger(logger *slog.Logger) {
	i.logger = logger
}

func (i *Implements[T]) setExec(fn context.CancelFunc) {
	i.exec = fn
}

func (i *Implements[T]) Exec() {
	i.exec()
}

func (Implements[T]) implements(T) {}
