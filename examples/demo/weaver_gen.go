// Code generated by "weaver generate". DO NOT EDIT.
//go:build !ignoreWeaverGen

package main

import (
	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/runtime/codegen"
	"reflect"
)

func init() {
	codegen.Register(codegen.Registration{
		Name:  "github.com/jun3372/weaver/Main",
		Interface: reflect.TypeOf((*weaver.Main)(nil)).Elem(),
		Impl:  reflect.TypeOf(app{}),
	})
}
