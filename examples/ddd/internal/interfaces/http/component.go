package httpapi

import (
	"context"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/ddd/internal/application"
)

// T 是 HTTP 交付组件的接口;交付层只通过应用服务与领域交互。
type T any

type impl struct {
	weaver.Implements[T]
	weaver.Listener[weaver.Handler] `conf:"http"`
	users                           weaver.Ref[application.UserService]
	orders                          weaver.Ref[application.OrderService]
}

// Init 注册路由即可,监听与优雅关闭由 Listener 托管,无需实现 Start。
func (i *impl) Init(ctx context.Context) error {
	h := newHandler(i.users.Get(), i.orders.Get(), i.Logger(ctx))
	i.Listener.Handler(h.routes())
	return nil
}
