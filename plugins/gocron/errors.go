package gocron

import "github.com/pkg/errors"

var (
	ErrNotEnabled      = errors.New("gocron not enabled")
	ErrNotReady        = errors.New("gocron not ready")
	ErrInvalidTimezone = errors.New("invalid timezone")
)
