package gocron

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"time"
	"unsafe"

	gocronv2 "github.com/go-co-op/gocron/v2"
	"github.com/pkg/errors"
)

func newTestScheduler(t *testing.T, conf Config) *scheduler {
	t.Helper()
	s := &scheduler{}
	s.SetConfig(conf)
	// setLogger 未导出且定义在 weaver 包，测试里经反射注入
	f := reflect.ValueOf(s).Elem().FieldByName("Implements").FieldByName("logger")
	reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().
		Set(reflect.ValueOf(slog.Default()))
	return s
}

func noop() {}

func TestClient_Disabled(t *testing.T) {
	s := newTestScheduler(t, Config{Enabled: false})

	_, err := s.Client(context.Background())
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Client err = %v, want ErrNotEnabled", err)
	}
}

func TestClient_NotReady(t *testing.T) {
	s := newTestScheduler(t, Config{Enabled: true})

	_, err := s.Client(context.Background())
	if !errors.Is(err, ErrNotReady) {
		t.Errorf("Client err = %v, want ErrNotReady", err)
	}
}

func TestRegister_Disabled(t *testing.T) {
	s := newTestScheduler(t, Config{Enabled: false})

	err := s.Register(context.Background(), gocronv2.DurationJob(time.Hour), gocronv2.NewTask(noop))
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Register err = %v, want ErrNotEnabled", err)
	}
}

func TestRegister_NotReady(t *testing.T) {
	s := newTestScheduler(t, Config{Enabled: true})

	err := s.Register(context.Background(), gocronv2.DurationJob(time.Hour), gocronv2.NewTask(noop))
	if !errors.Is(err, ErrNotReady) {
		t.Errorf("Register err = %v, want ErrNotReady", err)
	}
}

func TestInit_Disabled(t *testing.T) {
	s := newTestScheduler(t, Config{Enabled: false})

	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init err = %v, want nil", err)
	}
	if s.sched != nil {
		t.Error("Init 应该不创建 scheduler")
	}
}

func TestInit_InvalidTimezone(t *testing.T) {
	s := newTestScheduler(t, Config{Enabled: true, Timezone: "Invalid/Zone"})

	if err := s.Init(context.Background()); !errors.Is(err, ErrInvalidTimezone) {
		t.Fatalf("Init err = %v, want ErrInvalidTimezone", err)
	}
	if s.sched != nil {
		t.Error("Init 失败后不应留下 scheduler")
	}
}

func TestInit_Ok(t *testing.T) {
	s := newTestScheduler(t, Config{Enabled: true})

	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init err = %v, want nil", err)
	}
	if s.sched == nil {
		t.Fatal("Init 后 sched 不应该为 nil")
	}
	if err := s.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestRegister_Ok(t *testing.T) {
	s := newTestScheduler(t, Config{Enabled: true})
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init err = %v, want nil", err)
	}
	defer s.Shutdown()

	err := s.Register(context.Background(),
		gocronv2.DurationJob(time.Hour),
		gocronv2.NewTask(noop),
		gocronv2.WithName("test-job"),
	)
	if err != nil {
		t.Fatalf("Register err = %v, want nil", err)
	}
	if got := len(s.sched.Jobs()); got != 1 {
		t.Errorf("Jobs 数量 = %d, want 1", got)
	}
}

func TestRegister_InvalidDefinition(t *testing.T) {
	s := newTestScheduler(t, Config{Enabled: true})
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init err = %v, want nil", err)
	}
	defer s.Shutdown()

	err := s.Register(context.Background(), gocronv2.CronJob("invalid", false), gocronv2.NewTask(noop))
	if err == nil {
		t.Fatal("Register 非法 crontab 应该报错")
	}
	if got := len(s.sched.Jobs()); got != 0 {
		t.Errorf("注册失败后 Jobs 数量 = %d, want 0", got)
	}
}

func TestShutdown_NoScheduler(t *testing.T) {
	s := newTestScheduler(t, Config{})

	if err := s.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestTimezoneFor(t *testing.T) {
	tests := []struct {
		name    string
		tz      string
		want    *time.Location
		wantErr error
	}{
		{name: "空串回退本地", tz: "", want: time.Local},
		{name: "IANA 时区", tz: "Asia/Shanghai", want: mustLocation(t, "Asia/Shanghai")},
		{name: "UTC", tz: "UTC", want: time.UTC},
		{name: "非法时区", tz: "Mars/Olympus", wantErr: ErrInvalidTimezone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timezoneFor(tt.tz)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("timezoneFor(%q) err = %v, want %v", tt.tz, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("timezoneFor(%q) unexpected err: %v", tt.tz, err)
			}
			if got.String() != tt.want.String() {
				t.Errorf("timezoneFor(%q) = %v, want %v", tt.tz, got, tt.want)
			}
		})
	}
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("加载时区 %q 失败: %v", name, err)
	}
	return loc
}
