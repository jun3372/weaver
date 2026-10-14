package mongodb

import "github.com/pkg/errors"

var (
	ErrNotEnabled = errors.New("mongodb not enabled")
	ErrNotReady   = errors.New("mongodb not ready")
)
