package zookeeper

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("zookeeper not enabled")
	ErrNotReady   = errors.New("zookeeper not ready")
	ErrInvalidDSN = errors.New("invalid dsn")
)
