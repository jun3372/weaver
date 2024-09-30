package user

import (
	"context"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/hello/chat"
)

type User interface {
	SayHello(ctx context.Context, name string) (response, error)
}

type user struct {
	weaver.Implements[User]
	weaver.WithConfig[option] `weaver:"user"`
	weaver.Ref[chat.Chat]
}

type option struct {
	Source string
	Type   string
}
type response struct {
	Message string
	Option  option
}

func (u *user) Init(ctx context.Context) error {
	u.Logger(ctx).Info("user init")
	return nil
}

func (u *user) Shutdown(ctx context.Context) error {
	u.Logger(ctx).Warn("user Shutdown")
	return nil
}

func (u *user) SayHello(ctx context.Context, name string) (response, error) {
	u.Logger(ctx).Info("user SayHello", "name", name)
	return response{
		Message: "Hello " + name,
		Option:  *u.Config(),
	}, nil
}
