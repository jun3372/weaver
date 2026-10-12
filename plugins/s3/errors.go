package s3

import "github.com/pkg/errors"

var (
	ErrNotEnabled     = errors.New("s3 not enabled")
	ErrNotReady       = errors.New("s3 not ready")
	ErrInvalidDSN     = errors.New("invalid dsn")
	ErrInvalidScheme  = errors.New("invalid scheme")
	ErrBucketNotFound = errors.New("bucket not found")
)
