package wechat

import "github.com/jun3372/weaver"

type T interface {
}

type impl struct {
	weaver.Implements[T]
}
