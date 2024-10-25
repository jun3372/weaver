package main

import (
	"testing"
)

func TestMain(t *testing.T) {
	// os.Setenv("SERVICE_CONFIG", "/home/zhoujun/code/jun3/golang/github.com/cotton-go/weaver/examples/hello/weaver.yaml")
	if err := run(); err != nil {
		t.Fatal(err)
	}
}
