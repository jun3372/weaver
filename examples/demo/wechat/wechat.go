package wechat

import (
	"context"

	"github.com/jun3372/weaver"
)

type option struct {
	AppName string
	Version string
}

type T interface {
	Get() option
}

type impl struct {
	weaver.Implements[T]
	weaver.WithConfig[option] `conf:"wechat"` // 配置文件路径
}

func (i *impl) Get() option {
	return option{}
}

func (i *impl) Init(ctx context.Context) error {
	i.Logger(ctx).Info("wechat init", "conf", i.Config())
	return nil
}

func (i *impl) Start(ctx context.Context) error {
	i.Logger(ctx).Info("wechat start")
	<-ctx.Done()
	return nil
}
