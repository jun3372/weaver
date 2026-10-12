package nacos

import "github.com/pkg/errors"

var (
	ErrNotEnabled    = errors.New("nacos not enabled")
	ErrNotReady      = errors.New("nacos not ready")
	ErrInvalidDSN    = errors.New("invalid dsn")
	ErrInvalidScheme = errors.New("invalid scheme")
)
