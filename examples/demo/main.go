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
	weaver.Ref[wechat.T]
	weaver.WithConfig[options] `conf:"app"`
	opt                        weaver.WithConfig[options] `conf:"app"`
}

func main() {
	err := weaver.Run(context.Background(), func(ctx context.Context, t *app) error {
		t.Logger(ctx).Info("hello world", "conf", t.Config(), "opt", t.opt.Config())
		// go func() {
		// 	time.Sleep(time.Second * 5)
		// 	t.Logger(ctx).Info("开始退出", "time", time.Now())
		// 	t.Exec()
		// }()

		go func() {
			ticker := time.NewTicker(time.Second * 5)
			for {
				select {
				case <-ticker.C:
					opt, err := t.Get().Get(ctx)
					if err != nil {
						t.Logger(ctx).Error("wechat Get 失败", "err", err)
						continue
					}
					t.Logger(ctx).Info("hello world", "conf", t.Config(), "opt", t.opt.Config(), "wechat", opt)
				case <-ctx.Done():
					ticker.Stop()
					return
				}
			}
		}()
		<-ctx.Done()
		return nil
	})

	slog.Warn("main", "err", err)
}
