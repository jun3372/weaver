package wechat

import (
	"context"

	"github.com/jun3372/weaver"
)

type option struct{}

type T interface {
	Get() option
}

type impl struct {
	weaver.Implements[T]
}

func (i *impl) Get() option {
	return option{}
}

func (i *impl) Start(ctx context.Context) error {
	i.Logger(ctx).Info("wechat start")
	<-ctx.Done()
	return nil
}
