package main

import (
	"context"

	"github.com/jun3372/weaver"
)

type app struct {
	weaver.Implements[weaver.Main]
}

func main() {
	weaver.Run(context.Background(), func(ctx context.Context, t *app) error {
		return nil
	})
}
