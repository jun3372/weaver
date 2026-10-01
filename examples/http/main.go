package main

import (
	"context"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/http/server"
)

type option struct {
	Name string
}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.Ref[server.T]
	weaver.WithConfig[option] `conf:"app"`
}

func (a *app) Init(ctx context.Context) error {
	a.Logger(ctx).Info("app init", "conf", a.Config())
	return nil
}

func (a *app) Shutdown(ctx context.Context) error {
	a.Logger(ctx).Info("app shutdown")
	return nil
}

func main() {
	if err := weaver.Run(context.Background(), func(ctx context.Context, a *app) error {
		a.Logger(ctx).Info("http 示例已启动:修改 etc/weaver.yaml 中 http.Message 后,无需重启即可在响应中看到新值")
		<-ctx.Done()
		return nil
	}); err != nil {
		panic(err)
	}
}
