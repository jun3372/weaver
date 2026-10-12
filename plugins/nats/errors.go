package nats

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("nats not enabled")
	ErrNotReady   = errors.New("nats not ready")
)
