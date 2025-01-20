package logger

import (
	"context"
	"log/slog"
)

type Logger interface {
	Logger(context.Context) *slog.Logger
}

type logger struct {
	slog    *slog.Logger
	options *Options
}

func New(opts ...Option) Logger {
	s := &logger{
		slog:    slog.Default(),
		options: newDefaultOptions(),
	}

	s.options.Apply(opts...)
	s.slog = slog.New(s.options.getHandler())
	return s
}

func (s *logger) Logger(context.Context) *slog.Logger {
	return s.slog
}
