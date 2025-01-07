package chat

import (
	"context"

	"github.com/jun3372/weaver"
)

type Chat interface{}

type option struct {
	AppName string
	Version string
}

type chat struct {
	weaver.Implements[Chat]
	weaver.WithConfig[option] `conf:"chat"`
}

func (app *chat) Init(ctx context.Context) error {
	app.Logger(ctx).Info("Chat init")
	return nil
}

func (app *chat) Start(ctx context.Context) error {
	app.Logger(ctx).Info("Chat Start")
	<-ctx.Done()
	return nil
}

func (app *chat) Shutdown(ctx context.Context) error {
	app.Logger(ctx).Warn("Chat Shutdown")
	return nil
}
