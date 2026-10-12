package elasticsearch

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("elasticsearch not enabled")
	ErrNotReady   = errors.New("elasticsearch not ready")
	ErrInvalidDSN = errors.New("invalid dsn")
)
