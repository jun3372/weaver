package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/demo/wechat"
)

type options struct {
	Name string `yaml:"name" json:"name"`
}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.Ref[wechat.T]
	weaver.WithConfig[options]
	opt weaver.WithConfig[options] `toml:"app"`
}

func main() {
	err := weaver.Run(context.Background(), func(ctx context.Context, t *app) error {
		t.Logger(ctx).Info("hello world", "conf", t.Config(), "opt", t.opt.Config())
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
