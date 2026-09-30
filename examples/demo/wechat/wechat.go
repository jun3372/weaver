package wechat

import (
	"context"

	"github.com/jun3372/weaver"
)

type option struct {
	AppID     string
	AppSecret string
	Version   string
}

type T interface {
	Get(ctx context.Context) (option, error)
}

type impl struct {
	weaver.Implements[T]
	weaver.WithConfig[option] `conf:"wechat"` // 配置文件路径
}

func (i *impl) Get(ctx context.Context) (option, error) {
	return i.Config(), nil
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
