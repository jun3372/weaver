package main

import (
	"testing"
)

func TestMain(t *testing.T) {
	t.Setenv("SERVICE_CONFIG", "weaver.yaml")
	if err := run(); err != nil {
		t.Fatal(err)
	}
}
