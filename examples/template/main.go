package main

import (
	"context"
	"log/slog"
	"reflect"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/runtime/codegen"
)

type app struct {
	weaver.Implements[weaver.Main]
}

func main() {
	err := weaver.Run(context.Background(), func(ctx context.Context, t *app) error {
		t.Logger(ctx).Info("hello world")
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
