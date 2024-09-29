package chat

import (
	"reflect"

	"github.com/jun3372/weaver/runtime/codegen"
)

func init() {
	codegen.Register(codegen.Registration{
		Name:  "github.com/jun3372/weaver/examples/hello/chat.chat",
		Iface: reflect.TypeOf((*Chat)(nil)).Elem(),
		Impl:  reflect.TypeOf(chat{}),
	})
}
