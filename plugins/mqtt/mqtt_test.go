package mqtt

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"unsafe"

	"github.com/eclipse/paho.mqtt.golang"
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
	d := newTestDB(t, Config{Enabled: true, DSN: "http://localhost:1883"})

	if err := d.Init(context.Background()); !errors.Is(err, ErrInvalidScheme) {
		t.Fatalf("Init err = %v, want ErrInvalidScheme", err)
	}
	if d.cli != nil {
		t.Error("Init 失败后不应留下 client")
	}
}

func TestInit_PingFailure(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "mqtt://localhost:1"})

	if err := d.Init(context.Background()); err == nil {
		t.Skip("localhost:1 竟然连通了，跳过")
	}
	if d.cli != nil {
		t.Error("Connect 失败后 cli 不应该被赋值")
	}
}

func TestShutdown_NoClient(t *testing.T) {
	d := newTestDB(t, Config{})

	if err := d.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestValidateDSN(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr error
	}{
		{name: "mqtt", dsn: "mqtt://localhost:1883"},
		{name: "tcp", dsn: "tcp://localhost:1883"},
		{name: "ssl", dsn: "ssl://localhost:8883"},
		{name: "ws", dsn: "ws://localhost:8083/mqtt"},
		{name: "带 userinfo", dsn: "mqtt://user:pass@localhost:1883"},
		{name: "无 scheme", dsn: "localhost:1883", wantErr: ErrInvalidDSN},
		{name: "错误 scheme", dsn: "http://localhost:1883", wantErr: ErrInvalidScheme},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDSN(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("validateDSN(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateDSN(%q) unexpected err: %v", tt.dsn, err)
			}
		})
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	if err := validateDSN("mqtt://localhost:1883"); err != nil {
		t.Errorf("默认 DSN 校验失败: %v", err)
	}
}

func TestNewClientOptions(t *testing.T) {
	opts := mqtt.NewClientOptions()
	if opts == nil {
		t.Fatal("mqtt.NewClientOptions() = nil")
	}
}
