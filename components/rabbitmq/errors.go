package rabbitmq

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("rabbitmq not enabled")
	ErrNotReady   = errors.New("rabbitmq not ready")
)
