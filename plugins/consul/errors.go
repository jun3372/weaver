package consul

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("consul not enabled")
	ErrNotReady   = errors.New("consul not ready")
	ErrInvalidDSN = errors.New("invalid dsn")
)
