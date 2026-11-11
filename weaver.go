package weaver

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
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
	flagsOnce     sync.Once
	confFile      string
	showVer       bool
	envPrefixName string // 环境变量前缀,默认 "WEAVER",可由 SERVICE_ENV_PREFIX 覆盖
)

func parseFlags() {
	flagsOnce.Do(func() {
		flag.StringVar(&confFile, "conf", os.Getenv("SERVICE_CONFIG"), "config file path")
		flag.BoolVar(&showVer, "version", strings.ToLower(os.Getenv("SERVICE_VERSION")) == "true", "print version info")
		resolveEnvPrefix()
	})
	// 宿主程序可能已自行调用过 flag.Parse
	if !flag.Parsed() {
		flag.Parse()
	}
}

// resolveEnvPrefix 从 SERVICE_ENV_PREFIX 读取环境变量前缀,缺省 "WEAVER"。
func resolveEnvPrefix() {
	envPrefixName = os.Getenv("SERVICE_ENV_PREFIX")
	if envPrefixName == "" {
		envPrefixName = "WEAVER"
	}
}

var confNames = []string{"weaver", "config"}
var confExts = []string{"yaml", "yml", "toml", "json"}

// discoverConf 依次在 dirs 中查找配置文件,目录顺序优先于文件名,文件名
// 顺序(weaver → config)优先于扩展名(yaml → yml → toml → json),
// 返回首个命中的完整路径,未命中返回空串。
func discoverConf(dirs []string) string {
	for _, dir := range dirs {
		for _, name := range confNames {
			for _, ext := range confExts {
				p := filepath.Join(dir, name+"."+ext)
				if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
					return p
				}
			}
		}
	}
	return ""
}

func Run[T any, P PointerToMain[T]](ctx context.Context, app func(context.Context, *T) error) error {
	parseFlags()

	if showVer {
		version.PrintVersion()
		return nil
	}

	var conf *viper.Viper
	if confFile == "" {
		var dirs []string
		if wd, err := os.Getwd(); err == nil {
			dirs = append(dirs, wd)
		}
		if exe, err := os.Executable(); err == nil {
			dirs = append(dirs, filepath.Dir(exe))
		}
		if p := discoverConf(dirs); p != "" {
			slog.Info("自动发现配置文件", "file", p)
			confFile = p
		}
	}
	if confFile != "" {
		conf = viper.New()
		// 环境变量不由 viper 解析:env 覆盖统一走 applyEnvVars(注入时反射读取
		// os.Getenv),此处不启用 AutomaticEnv/BindEnv,避免多条解析通道并存。
		conf.SetConfigFile(confFile)
		if err := conf.ReadInConfig(); err != nil {
			return errors.Errorf("Fatal error config file: %v", err)
		}
	}

	var cancel context.CancelFunc
	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	widget := newWidget(ctx, cancel, conf, codegen.Registered(), envPrefixName)
	main, err := widget.getImpl(reflection.Type[T]())
	if err != nil {
		return err
	}

	// 启动组件。
	//
	// 错误契约：start 只等待全部启动动作发起即返回,不等待长驻 Start,
	// 其返回值仅覆盖同步即可见的装配类失败(Listeners 扫描、Start/Listener
	// 冲突);Start/serve 返回错误或 panic 的完整上报链路为:错误写入 widget
	// 启动错误并 cancel ctx → app 退出 → shutdown(先 startWG.Wait,保证
	// 错误必然已写入)→ Run 优先返回启动错误。启动期错误不再依赖
	// goroutine 调度时序,也无需 app 主动监听 ctx 才能暴露。
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

	// 启动期错误优先于 app 的返回值:任一 Start/serve 失败会 cancel ctx,
	// app(如 RunComponent)返回的多是归因于该取消的错误,此时应返回
	// 真正的启动失败原因。无启动错误时原样返回 app 的结果。
	if startErr := widget.startupError(); startErr != nil {
		return startErr
	}
	return err
}

// RunComponent 启动全部组件后不执行额外主逻辑,阻塞等待退出信号
// (SIGINT/SIGQUIT/SIGTERM)或组件调用 Exit(),随后优雅关闭并返回 nil。
// Ctrl-C 等主动退出与 Exit() 属于正常关闭路径,context.Canceled /
// context.DeadlineExceeded 亦归一化为 nil;真正的错误(如启动失败)
// 原样返回,调用方只需区分"是否出错",无需识别退出原因。
// app 仅用于类型推断,传 (*T)(nil) 即可:
//
//	if err := weaver.RunComponent(ctx, (*app)(nil)); err != nil {
//		fmt.Fprintln(os.Stderr, err)
//		os.Exit(1)
//	}
func RunComponent[T any, P PointerToMain[T]](ctx context.Context, app *T) error {
	err := Run[T, P](ctx, func(ctx context.Context, _ *T) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
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
	exit   context.CancelFunc

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

func (i *Implements[T]) setExit(fn context.CancelFunc) {
	i.exit = fn
}

// Exit 主动触发整个应用的优雅退出，等价于向进程发送退出信号。
func (i *Implements[T]) Exit() {
	i.exit()
}

func (Implements[T]) implements(T) {}
