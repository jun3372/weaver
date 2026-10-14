package nsq

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("nsq not enabled")
	ErrNotReady   = errors.New("nsq not ready")
	ErrInvalidDSN = errors.New("invalid dsn")
)
