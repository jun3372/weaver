// Code generated by "weaver generate". DO NOT EDIT.
//go:build !ignoreWeaverGen

package wechat

import (
	"github.com/jun3372/weaver/runtime/codegen"
	"reflect"
)

func init() {
	codegen.Register(codegen.Registration{
		Name:      "github.com/jun3372/weaver/examples/demo/wechat/T",
		Interface: reflect.TypeOf((*T)(nil)).Elem(),
		Impl:      reflect.TypeOf(impl{}),
	})
}

// Check that impl implements the T interface.
var _ T = (*impl)(nil)

