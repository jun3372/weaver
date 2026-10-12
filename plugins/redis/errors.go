package redis

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("redis not enabled")
	ErrNotReady   = errors.New("redis not ready")
)
