package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/demo/wechat"
)

type options struct {
	Name string
}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.WithConfig[options] `weaver:"app"`
	weaver.Ref[wechat.T]
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