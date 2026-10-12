package db

import "github.com/pkg/errors"

var (
	ErrNotReady      = errors.New("db not ready")
	ErrNotEnabled    = errors.New("db not enabled")
	ErrInvalidDSN    = errors.New("invalid dsn")
	ErrInvalidScheme = errors.New("invalid scheme")
)
