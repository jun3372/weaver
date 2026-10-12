package kafka

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("kafka not enabled")
	ErrNotReady   = errors.New("kafka not ready")
	ErrInvalidDSN = errors.New("invalid dsn")
)
