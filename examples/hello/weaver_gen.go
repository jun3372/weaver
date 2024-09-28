package main

import (
	"reflect"

	"github.com/jun3372/weaver/runtime/codegen"
)

func init() {
	codegen.Register(codegen.Registration{
		Name: "github.com/jun3372/weaver/examples/hello.app",
		Impl: reflect.TypeOf(app{}),
	})
}
