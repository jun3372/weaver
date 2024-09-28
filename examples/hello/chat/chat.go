package chat

import (
	"context"
	"log/slog"

	"github.com/jun3372/weaver"
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
