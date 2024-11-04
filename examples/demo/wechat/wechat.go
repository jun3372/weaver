package wechat

import "github.com/jun3372/weaver"

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
