// Code generated by "weaver generate". DO NOT EDIT.
//go:build !ignoreWeaverGen

package chat

import (
	"github.com/jun3372/weaver/runtime/codegen"
	"reflect"
)

func init() {
	codegen.Register(codegen.Registration{
		Name:  "github.com/jun3372/weaver/examples/hello/chat/Chat",
		Interface: reflect.TypeOf((*Chat)(nil)).Elem(),
		Impl:  reflect.TypeOf(chat{}),
	})
}
