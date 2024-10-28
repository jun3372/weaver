package main

import (
	"context"
	"log/slog"
	"reflect"
	"time"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/runtime/codegen"
)

type options struct {
	Name string
}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.WithConfig[options] `weaver:"app"`
}

func main() {
	err := weaver.Run(context.Background(), func(ctx context.Context, t *app) error {
		t.Logger(ctx).Info("hello world", "conf", t.Config())
		go func() {
			time.Sleep(time.Second * 5)
			t.Logger(ctx).Info("开始退出", "time", time.Now())
			t.Exec()
		}()
		<-ctx.Done()
		return nil
	})

	slog.Warn("main", "err", err)
}

func init() {
	codegen.Register(codegen.Registration{
		Name:  "github.com/jun3372/weaver/examples/hello.app",
		Iface: reflect.TypeOf((*weaver.Main)(nil)).Elem(),
		Impl:  reflect.TypeOf(app{}),
	})
}
