package db

import (
	"context"
	"log/slog"
	"time"

	"github.com/pkg/errors"
	gormlogger "gorm.io/gorm/logger"
)

// slogLogger 把 GORM 的日志桥接到 weaver 的 slog（Logger(ctx) 自动附加 trace/span ID）。
// 默认级别与慢查询阈值沿用 gorm 默认值（Warn / 200ms）。
type slogLogger struct {
	logger        func(context.Context) *slog.Logger
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

func newSlogLogger(logger func(context.Context) *slog.Logger) *slogLogger {
	return &slogLogger{
		logger:        logger,
		level:         gormlogger.Warn,
		slowThreshold: 200 * time.Millisecond,
	}
}

func (l *slogLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	n := *l
	n.level = level
	return &n
}

func (l *slogLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.level < gormlogger.Info {
		return
	}
	l.logger(ctx).InfoContext(ctx, msg, args...)
}

func (l *slogLogger) Warn(ctx context.Context, msg string, args ...any) {
	if l.level < gormlogger.Warn {
		return
	}
	l.logger(ctx).WarnContext(ctx, msg, args...)
}

func (l *slogLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.level < gormlogger.Error {
		return
	}
	l.logger(ctx).ErrorContext(ctx, msg, args...)
}

func (l *slogLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	sql, rows := fc()
	elapsed := time.Since(begin)
	lg := l.logger(ctx)

	switch {
	case err != nil && !errors.Is(err, gormlogger.ErrRecordNotFound) && l.level >= gormlogger.Error:
		lg.ErrorContext(ctx, "gorm 查询失败", "sql", sql, "rows", rows, "elapsed", elapsed, "error", err)
	case l.slowThreshold > 0 && elapsed > l.slowThreshold && l.level >= gormlogger.Warn:
		lg.WarnContext(ctx, "gorm 慢查询", "sql", sql, "rows", rows, "elapsed", elapsed, "threshold", l.slowThreshold)
	case l.level >= gormlogger.Info:
		lg.DebugContext(ctx, "gorm 查询", "sql", sql, "rows", rows, "elapsed", elapsed)
	}
}
