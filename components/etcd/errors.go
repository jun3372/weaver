package etcd

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("etcd not enabled")
	ErrNotReady   = errors.New("etcd not ready")
	ErrInvalidDSN = errors.New("invalid dsn")
)
