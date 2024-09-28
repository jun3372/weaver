package main

import (
	"reflect"

	"github.com/cotton-go/weaver/runtime/codegen"
)

func init() {
	codegen.Register(codegen.Registration{
		Name: "hello.app",
		Impl: reflect.TypeOf(app{}),
	})
	codegen.Register(codegen.Registration{
		Name: "hello.chat",
		Impl: reflect.TypeOf(Chat{}),
	})
}
