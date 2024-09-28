package user

import (
	"context"
	"log/slog"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/hello/chat"
)

type User struct {
	weaver.Ref[chat.Chat]
}

func (u *User) Init(ctx context.Context) error {
	slog.Info("user init")
	return nil
}

func (u *User) Shutdown(ctx context.Context) error {
	slog.Info("user Shutdown")
	return nil
}

func (u *User) SayHello(ctx context.Context, name string) (string, error) {
	return "hello:" + name, nil
}
