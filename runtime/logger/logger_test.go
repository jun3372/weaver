package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestLogger(t *testing.T) {
	l := New(WithLevel(slog.LevelDebug), WithType("json"), WithAddSource(true), WithFilename("./test.log"))
	l.Logger(context.Background()).Debug("test")
	l.Logger(context.Background()).Info("test")
	l.Logger(context.Background()).Warn("test")
	l.Logger(context.Background()).Error("test")
}
