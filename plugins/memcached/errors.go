package memcached

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("memcached not enabled")
	ErrNotReady   = errors.New("memcached not ready")
	ErrInvalidDSN = errors.New("invalid dsn")
)
