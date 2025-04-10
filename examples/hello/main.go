package main

import (
	"context"
	"time"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/hello/chat"
	"github.com/jun3372/weaver/examples/hello/user"
)

type option struct {
	AppName string
	Version string
}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.WithConfig[option] `conf:"app"`
	weaver.Ref[user.User]
	chat weaver.Ref[chat.Chat]
}

func (app *app) Init(ctx context.Context) error {
	app.Logger(ctx).Info("App init")
	return nil
}

func (app *app) Shutdown(ctx context.Context) error {
	app.Logger(ctx).Info("App Shutdown")
	return nil
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	return weaver.Run(context.Background(), func(ctx context.Context, app *app) error {
		// app.Logger(ctx).Info("App run")
		// <-ctx.Done()
		// return nil
		// {
		// 	resp, err := app.u.Get().SayHello(ctx, "jun3372")
		// 	if err != nil {
		// 		return err
		// 	}

		// 	app.Logger(ctx).Info("resp", "msg", resp)
		// }
		{
			resp, err := app.Get().SayHello(ctx, "jun3372")
			if err != nil {
				return err
			}

			app.Logger(ctx).Info("resp", "msg", resp)
		}

		ctx, cannel := context.WithCancel(ctx)
		go func() {
			time.Sleep(time.Second * 5)
			app.Logger(ctx).Info("on cannel")
			cannel()
		}()

		<-ctx.Done()
		return nil
	})
}
