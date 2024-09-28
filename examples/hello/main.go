package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/hello/user"
)

type option struct {
	AppName string
	Version string
}

type app struct {
	weaver.WithConfig[option] `weaver:"app"`
	// weaver.Ref[chat.Chat]
	weaver.Ref[user.User]
}

func (app *app) Init(context.Context) error {
	slog.Info("app init")
	return nil
}

func (app *app) Shutdown(context.Context) error {
	slog.Info("app Shutdown")
	return nil
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	return weaver.Run(context.Background(), func(ctx context.Context, app *app) error {
		slog.Info("hello", "conf", app.Config(), "u", app.Get())

		app.Get().SayHello(ctx, "jun3372")
		ctx, cannel := context.WithCancel(ctx)
		go func() {
			time.Sleep(time.Second * 5)
			slog.Info("on cannel")
			cannel()
		}()

		<-ctx.Done()
		return nil
	})
}
