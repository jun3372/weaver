package db

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func reflectLoggerField(t *testing.T, d *db) reflect.Value {
	t.Helper()
	// setLogger 未导出且定义在 weaver 包，测试里经反射注入
	f := reflect.ValueOf(d).Elem().FieldByName("Implements").FieldByName("logger")
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
}

func newCaptureLogger(t *testing.T) (func(context.Context) *slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return func(context.Context) *slog.Logger { return slog.New(h) }, &buf
}

func testAdapter(t *testing.T) (*slogLogger, *bytes.Buffer) {
	t.Helper()
	fn, buf := newCaptureLogger(t)
	return newSlogLogger(fn), buf
}

func TestLogMode(t *testing.T) {
	l, _ := testAdapter(t)

	got := l.LogMode(gormlogger.Info)
	if got == gormlogger.Interface(l) {
		t.Fatal("LogMode 应该返回新实例，不修改原实例")
	}
	if l.level != gormlogger.Warn {
		t.Errorf("原实例 level = %v, want Warn（不应被修改）", l.level)
	}
	if got.(*slogLogger).level != gormlogger.Info {
		t.Errorf("新实例 level = %v, want Info", got.(*slogLogger).level)
	}
}

func TestInfo_Warn_Error(t *testing.T) {
	tests := []struct {
		name    string
		call    func(l *slogLogger, ctx context.Context)
		level   gormlogger.LogLevel
		wantSub string
	}{
		{
			name:    "Info 达标",
			call:    func(l *slogLogger, ctx context.Context) { l.Info(ctx, "hello %s", "world") },
			level:   gormlogger.Info,
			wantSub: "level=INFO msg=\"hello world\"",
		},
		{
			name:  "Info 低于级别",
			call:  func(l *slogLogger, ctx context.Context) { l.Info(ctx, "hidden") },
			level: gormlogger.Warn,
		},
		{
			name:    "Warn 达标",
			call:    func(l *slogLogger, ctx context.Context) { l.Warn(ctx, "careful") },
			level:   gormlogger.Warn,
			wantSub: "level=WARN msg=careful",
		},
		{
			name:  "Warn 低于级别",
			call:  func(l *slogLogger, ctx context.Context) { l.Warn(ctx, "hidden") },
			level: gormlogger.Error,
		},
		{
			name:    "Error 达标",
			call:    func(l *slogLogger, ctx context.Context) { l.Error(ctx, "boom") },
			level:   gormlogger.Error,
			wantSub: "level=ERROR msg=boom",
		},
		{
			name:  "Error 静默",
			call:  func(l *slogLogger, ctx context.Context) { l.Error(ctx, "hidden") },
			level: gormlogger.Silent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, buf := testAdapter(t)
			l.level = tt.level

			ctx := context.Background()
			tt.call(l, ctx)
			out := buf.String()
			if tt.wantSub != "" {
				if !strings.Contains(out, tt.wantSub) {
					t.Errorf("输出 = %q, want 含 %q", out, tt.wantSub)
				}
				return
			}
			if out != "" {
				t.Errorf("低于级别时不应输出, got %q", out)
			}
		})
	}
}

func TestTrace_Error(t *testing.T) {
	l, buf := testAdapter(t)
	l.level = gormlogger.Error

	l.Trace(context.Background(), time.Now(), func() (string, int64) { return "SELECT 1", 0 },
		errors.New("boom"))

	out := buf.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "gorm 查询失败") || !strings.Contains(out, "SELECT 1") {
		t.Errorf("输出 = %q, want ERROR 级别的查询失败日志", out)
	}
}

func TestTrace_RecordNotFound(t *testing.T) {
	l, buf := testAdapter(t)
	l.level = gormlogger.Error

	l.Trace(context.Background(), time.Now(), func() (string, int64) { return "SELECT 1", 0 },
		gormlogger.ErrRecordNotFound)

	if strings.Contains(buf.String(), "level=ERROR") {
		t.Errorf("ErrRecordNotFound 不应按 ERROR 记录, got %q", buf.String())
	}
}

func TestTrace_Slow(t *testing.T) {
	l, buf := testAdapter(t)
	l.level = gormlogger.Warn

	l.Trace(context.Background(), time.Now().Add(-300*time.Millisecond), func() (string, int64) { return "SELECT 1", 1 }, nil)

	out := buf.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "gorm 慢查询") {
		t.Errorf("输出 = %q, want WARN 级别的慢查询日志", out)
	}
}

func TestTrace_Normal(t *testing.T) {
	tests := []struct {
		name  string
		level gormlogger.LogLevel
		want  bool
	}{
		{name: "Info 级别记录", level: gormlogger.Info, want: true},
		{name: "Warn 级别忽略", level: gormlogger.Warn, want: false},
		{name: "Silent 忽略", level: gormlogger.Silent, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, buf := testAdapter(t)
			l.level = tt.level

			l.Trace(context.Background(), time.Now(), func() (string, int64) { return "SELECT 1", 1 }, nil)

			out := buf.String()
			if tt.want && !strings.Contains(out, "gorm 查询") {
				t.Errorf("输出 = %q, want 含普通查询日志", out)
			}
			if !tt.want && out != "" {
				t.Errorf("输出 = %q, want 空", out)
			}
		})
	}
}

// 端到端：Init 注入适配器后，实际查询日志经 weaver logger 输出。
func TestInit_InjectsSlogLogger(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	d := &db{}
	d.SetConfig(Config{Enabled: true, DSN: "sqlite://:memory:"})
	f := reflectLoggerField(t, d)
	f.Set(reflect.ValueOf(slog.New(h)))

	if err := d.Init(context.Background()); err != nil {
		t.Fatalf("Init err = %v, want nil", err)
	}
	if d.db == nil {
		t.Fatal("Init 后 d.db = nil")
	}
	if d.db.Logger == nil {
		t.Fatal("gorm Logger 未注入")
	}
	if _, ok := d.db.Logger.(*slogLogger); !ok {
		t.Fatalf("gorm Logger = %T, want *slogLogger", d.db.Logger)
	}

	// 用户自定义 Logger 不被覆盖
	custom := gormlogger.Default
	d2 := &db{}
	d2.SetConfig(Config{Enabled: true, DSN: "sqlite://:memory:", GormConfig: &gorm.Config{Logger: custom}})
	f2 := reflectLoggerField(t, d2)
	f2.Set(reflect.ValueOf(slog.New(h)))
	if err := d2.Init(context.Background()); err != nil {
		t.Fatalf("Init err = %v, want nil", err)
	}
	if d2.db.Logger != gormlogger.Interface(custom) {
		t.Errorf("用户自定义 Logger 被覆盖: %T", d2.db.Logger)
	}

	// 执行一次查询，确认 SQL 经 slog 输出（session 调到 Info 触发查询日志）
	session := d.db.Session(&gorm.Session{Logger: d.db.Logger.LogMode(gormlogger.Info)})
	if err := session.Exec("SELECT 1").Error; err != nil {
		t.Fatalf("Exec err = %v", err)
	}
	if !strings.Contains(buf.String(), "SELECT 1") {
		t.Errorf("查询日志未输出: %q", buf.String())
	}
}
