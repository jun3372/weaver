package consul

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"unsafe"

	"github.com/hashicorp/consul/api"
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

func TestInit_InvalidDSN(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "127.0.0.1:8500"})

	if err := d.Init(context.Background()); !errors.Is(err, ErrInvalidDSN) {
		t.Fatalf("Init err = %v, want ErrInvalidDSN", err)
	}
	if d.cli != nil {
		t.Error("Init 失败后不应留下 client")
	}
}

func TestInit_PingFailure(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "http://localhost:1"})

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

func TestConfigFor(t *testing.T) {
	tests := []struct {
		name       string
		dsn        string
		wantAddr   string
		wantScheme string
		wantUser   string
		wantPass   string
		wantErr    error
	}{
		{name: "默认", dsn: "http://127.0.0.1:8500", wantAddr: "127.0.0.1:8500", wantScheme: "http"},
		{name: "https", dsn: "https://consul.example.com", wantAddr: "consul.example.com", wantScheme: "https"},
		{name: "带 userinfo", dsn: "http://admin:secret@127.0.0.1:8500", wantAddr: "127.0.0.1:8500", wantScheme: "http", wantUser: "admin", wantPass: "secret"},
		{name: "无 scheme", dsn: "127.0.0.1:8500", wantErr: ErrInvalidDSN},
		{name: "错误 scheme", dsn: "ftp://127.0.0.1:8500", wantErr: ErrInvalidDSN},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := configFor(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("configFor(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("configFor(%q) unexpected err: %v", tt.dsn, err)
			}
			if cfg.Address != tt.wantAddr || cfg.Scheme != tt.wantScheme {
				t.Errorf("configFor(%q) = %q/%q, want %q/%q", tt.dsn, cfg.Address, cfg.Scheme, tt.wantAddr, tt.wantScheme)
			}
			if tt.wantUser == "" {
				if cfg.HttpAuth != nil {
					t.Errorf("configFor(%q) HttpAuth = %v, want nil", tt.dsn, cfg.HttpAuth)
				}
				return
			}
			if cfg.HttpAuth == nil || cfg.HttpAuth.Username != tt.wantUser || cfg.HttpAuth.Password != tt.wantPass {
				t.Errorf("configFor(%q) HttpAuth = %v, want %q/%q", tt.dsn, cfg.HttpAuth, tt.wantUser, tt.wantPass)
			}
		})
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	cfg, err := configFor("http://127.0.0.1:8500")
	if err != nil {
		t.Errorf("默认 DSN 解析失败: %v", err)
	}
	if cfg.Address != "127.0.0.1:8500" || cfg.Scheme != "http" {
		t.Errorf("默认 DSN 解析结果 = %q/%q, want 127.0.0.1:8500/http", cfg.Address, cfg.Scheme)
	}
}

func TestDefaultConfig(t *testing.T) {
	if api.DefaultConfig() == nil {
		t.Fatal("api.DefaultConfig() = nil")
	}
}
