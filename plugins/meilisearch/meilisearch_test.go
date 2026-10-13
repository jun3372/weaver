package meilisearch

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"unsafe"

	"github.com/pkg/errors"
)

func newTestDB(t *testing.T, conf Config) *db {
	t.Helper()
	d := &db{}
	d.SetConfig(conf)
	// setLogger 未导出且定义在 weaver 包，测试里经反射注入
	f := reflect.ValueOf(d).Elem().FieldByName("Implements").FieldByName("logger")
	reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().
		Set(reflect.ValueOf(slog.Default()))
	return d
}

func TestClient_Disabled(t *testing.T) {
	d := newTestDB(t, Config{Enabled: false})

	_, err := d.Client(context.Background())
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Client err = %v, want ErrNotEnabled", err)
	}
}

func TestClient_NotReady(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true})

	_, err := d.Client(context.Background())
	if !errors.Is(err, ErrNotReady) {
		t.Errorf("Client err = %v, want ErrNotReady", err)
	}
}

func TestInit_Disabled(t *testing.T) {
	d := newTestDB(t, Config{Enabled: false})

	if err := d.Init(context.Background()); err != nil {
		t.Fatalf("Init err = %v, want nil", err)
	}
	if d.cli != nil {
		t.Error("Init 应该不创建 client")
	}
}

func TestInit_InvalidHost(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, Host: "localhost:7700"})

	if err := d.Init(context.Background()); !errors.Is(err, ErrInvalidHost) {
		t.Fatalf("Init err = %v, want ErrInvalidHost", err)
	}
	if d.cli != nil {
		t.Error("Init 失败后不应留下 client")
	}
}

func TestInit_HealthFailure(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, Host: "http://localhost:1"})

	if err := d.Init(context.Background()); err == nil {
		t.Skip("localhost:1 竟然连通了，跳过")
	}
	if d.cli != nil {
		t.Error("探活失败后 cli 不应该被赋值")
	}
}

func TestShutdown_NoClient(t *testing.T) {
	d := newTestDB(t, Config{})

	if err := d.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestHostFor(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		wantHost string
		wantErr  error
	}{
		{name: "http", host: "http://localhost:7700", wantHost: "http://localhost:7700"},
		{name: "https", host: "https://search.example.com", wantHost: "https://search.example.com"},
		{name: "带路径会剥掉", host: "http://localhost:7700/", wantHost: "http://localhost:7700"},
		{name: "无 scheme", host: "localhost:7700", wantErr: ErrInvalidHost},
		{name: "错误 scheme", host: "ftp://localhost:7700", wantErr: ErrInvalidHost},
		{name: "空 Host", host: "", wantErr: ErrInvalidHost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hostFor(tt.host)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("hostFor(%q) err = %v, want %v", tt.host, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("hostFor(%q) unexpected err: %v", tt.host, err)
			}
			if got != tt.wantHost {
				t.Errorf("hostFor(%q) = %q, want %q", tt.host, got, tt.wantHost)
			}
		})
	}
}

func TestDefaultHostParseable(t *testing.T) {
	got, err := hostFor("http://localhost:7700")
	if err != nil {
		t.Errorf("默认 Host 解析失败: %v", err)
	}
	if got != "http://localhost:7700" {
		t.Errorf("默认 Host 解析结果 = %q, want %q", got, "http://localhost:7700")
	}
}
