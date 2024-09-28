package main

import (
	"context"
	"log/slog"

	"github.com/cotton-go/weaver"
)

type Chat struct {
	weaver.WithConfig[option] `weaver:"chat"`
}

func (app *Chat) Init(context.Context) error {
	slog.Info("Chat init")
	return nil
}

func (app *Chat) Shutdown(context.Context) error {
	slog.Info("Chat Shutdown")
	return nil
}

type option struct {
	AppName string
	Version string
}

type app struct {
	weaver.WithConfig[option] `weaver:"app"`
	weaver.Ref[Chat]
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
		slog.Info("hello", "conf", app.Config(), "cgat", app.Get())
		return nil
	})
}
