package meilisearch

import "github.com/pkg/errors"

var (
	ErrNotEnabled  = errors.New("meilisearch not enabled")
	ErrNotReady    = errors.New("meilisearch not ready")
	ErrInvalidHost = errors.New("invalid host")
)
