package chat

import (
	"reflect"

	"github.com/jun3372/weaver/runtime/codegen"
)

func init() {
	codegen.Register(codegen.Registration{
		Name: "github.com/jun3372/weaver/examples/hello/chat.chat",
		Impl: reflect.TypeOf(Chat{}),
	})
}
