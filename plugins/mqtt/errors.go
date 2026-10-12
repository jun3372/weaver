package mqtt

import "github.com/pkg/errors"

var (
	ErrNotEnabled    = errors.New("mqtt not enabled")
	ErrNotReady      = errors.New("mqtt not ready")
	ErrInvalidDSN    = errors.New("invalid dsn")
	ErrInvalidScheme = errors.New("invalid scheme")
)
