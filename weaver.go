package weaver

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/cotton-go/weaver/internal/reflection"
	"github.com/cotton-go/weaver/runtime/codegen"
)

func Run[T any](ctx context.Context, app func(context.Context, *T) error) error {
	var cancel context.CancelFunc
	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	filename := os.Getenv("SERVICE_CONFIG")
	if filename == "" {
		return errors.New("no config file")
	}

	conf := viper.New()
	conf.SetConfigFile(filename)
	if err := conf.ReadInConfig(); err != nil {
		return errors.Errorf("Fatal error config file: %v", err)
	}

	regs := codegen.Registered()
	widg := newWidgrt(ctx, conf, regs)
	main, err := widg.getImpl(reflection.Type[T]())
	if err != nil {
		return err
	}

	err = app(ctx, main.(*T))
	cancel()
	widg.shutdown()
	return err
}

type WithConfig[T any] struct{ config T }

func (wc *WithConfig[T]) Config() *T { return &wc.config }

type Ref[T any] struct{ value T }

func (r Ref[T]) isRef()            {}
func (r Ref[T]) Get() T            { return r.value }
func (r *Ref[T]) setRef(value any) { r.value = value.(T) }

type Init interface {
	Init(context.Context) error
}

type Shutdown interface {
	Shutdown(context.Context) error
}
