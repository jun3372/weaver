package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jun3372/weaver"
	_ "github.com/jun3372/weaver/components/db"

	httpapi "github.com/jun3372/weaver/examples/ddd/internal/interfaces/http"
)

type option struct {
	Name string `default:"ddd-example"`
}

// app 是装配入口:领域层零框架依赖,应用层与基础设施层各自是组件,
// HTTP 交付层通过 Listener 托管监听。
type app struct {
	weaver.Implements[weaver.Main]
	weaver.WithConfig[option] `conf:"app"`
	server                    weaver.Ref[httpapi.T]
}

func (a *app) Init(ctx context.Context) error {
	a.Logger(ctx).Info("ddd 示例初始化", "app", a.Config().Name)
	return nil
}

func main() {
	if err := weaver.RunComponent(context.Background(), (*app)(nil)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
