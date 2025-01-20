package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Option func(*Options)

type Options struct {
	Level      slog.Level
	Type       string
	AddSource  bool
	Stdout     bool
	Filename   string
	MaxSize    int
	MaxAge     int
	MaxBackups int
	LocalTime  bool
	Compress   bool
}

func newDefaultOptions() *Options {
	return &Options{
		Level:      slog.LevelInfo,
		Type:       "console",
		Stdout:     true,
		Filename:   "",
		MaxSize:    100,
		MaxAge:     7,
		MaxBackups: 3,
		LocalTime:  true,
		Compress:   true,
	}
}

func (s *Options) Apply(opts ...Option) {
	for _, opt := range opts {
		opt(s)
	}
}

func WithLevel(value slog.Level) Option {
	return func(o *Options) {
		o.Level = value
	}
}

func WithLevelString(value string) Option {
	return func(o *Options) {
		switch strings.ToUpper(value) {
		case "DEBUG":
			o.Level = slog.LevelDebug
		case "INFO":
			o.Level = slog.LevelInfo
		case "WARN":
			o.Level = slog.LevelWarn
		case "ERROR":
			o.Level = slog.LevelError
		default:
			o.Level = slog.LevelInfo
		}
	}
}

func WithType(value string) Option {
	return func(o *Options) {
		o.Type = value
	}
}

func WithStdout(value bool) Option {
	return func(o *Options) {
		o.Stdout = value
	}
}

func WithFilename(value string) Option {
	return func(o *Options) {
		o.Filename = value
	}
}

func WithMaxSize(value int) Option {
	return func(o *Options) {
		o.MaxSize = value
	}
}

func WithMaxAge(value int) Option {
	return func(o *Options) {
		o.MaxAge = value
	}
}

func WithMaxBackups(value int) Option {
	return func(o *Options) {
		o.MaxBackups = value
	}
}

func WithLocalTime(value bool) Option {
	return func(o *Options) {
		o.LocalTime = value
	}
}

func WithCompress(value bool) Option {
	return func(o *Options) {
		o.Compress = value
	}
}

func WithAddSource(value bool) Option {
	return func(o *Options) {
		o.AddSource = value
	}
}

func (s *Options) getHandler() slog.Handler {
	opt := slog.HandlerOptions{Level: s.Level, AddSource: s.AddSource}
	switch s.Type {
	case "console", "text":
		return slog.NewTextHandler(s.getWriter(), &opt)
	case "json":
		return slog.NewJSONHandler(s.getWriter(), &opt)
	default:
		return slog.NewTextHandler(s.getWriter(), &opt)
	}
}

func (s *Options) getWriter() io.Writer {
	writers := make([]io.Writer, 0)
	if s.Stdout {
		writers = append(writers, os.Stdout)
	}

	if s.Filename != "" {
		writers = append(writers, &lumberjack.Logger{
			Filename:   s.Filename,
			LocalTime:  s.LocalTime,
			MaxSize:    s.MaxSize,
			MaxAge:     s.MaxAge,
			MaxBackups: s.MaxBackups,
			Compress:   s.Compress,
		})
	}

	return io.MultiWriter(writers...)
}
