package user

import (
	"reflect"

	"github.com/jun3372/weaver/runtime/codegen"
)

func init() {
	codegen.Register(codegen.Registration{
		Name:  "github.com/jun3372/weaver/examples/hello/user.User",
		Iface: reflect.TypeOf((*User)(nil)).Elem(),
		Impl:  reflect.TypeOf(user{}),
	})
}
