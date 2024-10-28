package weaver

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/jun3372/weaver/internal/reflection"
	"github.com/jun3372/weaver/runtime/codegen"
	"github.com/jun3372/weaver/version"
)

type Main interface{}

func Run[T any, P PointerToMain[T]](ctx context.Context, app func(context.Context, *T) error) error {
	var filename string
	var printVersion bool
	flag.StringVar(&filename, "conf", os.Getenv("SERVICE_CONFIG"), "config file path")
	flag.BoolVar(&printVersion, "version", strings.ToLower(os.Getenv("SERVICE_VERSION")) == "true", "print version info")
	flag.Parse()

	if printVersion {
		version.PrintVersion()
		return nil
	}

	var conf *viper.Viper
	if filename != "" {
		conf = viper.New()
		conf.SetConfigFile(filename)
		if err := conf.ReadInConfig(); err != nil {
			return errors.Errorf("Fatal error config file: %v", err)
		}
	}

	var cancel context.CancelFunc
	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	widg := newWidgrt(ctx, cancel, conf, codegen.Registered())
	main, err := widg.getImpl(reflection.Type[T]())
	if err != nil {
		return err
	}

	// 启动组件
	if err = widg.start(widg.ctx); err != nil {
		return err
	}

	if m, ok := main.(*T); !ok {
		return errors.New("main type error")
	} else {
		err = app(ctx, m)
	}

	cancel()
	widg.shutdown(context.Background())
	return err
}

type WithConfig[T any] struct{ config T }

func (wc *WithConfig[T]) Config() *T { return &wc.config }

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
	logger := i.logger
	// s := trace.SpanContextFromContext(ctx)
	// if s.HasTraceID() {
	// 	logger = logger.With("traceid", s.TraceID().String())
	// }
	// if s.HasSpanID() {
	// 	logger = logger.With("spanid", s.SpanID().String())
	// }
	return logger
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
