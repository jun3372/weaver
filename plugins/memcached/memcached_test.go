package memcached

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

func TestInit_InvalidDSN(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "localhost:11211"})

	if err := d.Init(context.Background()); !errors.Is(err, ErrInvalidDSN) {
		t.Fatalf("Init err = %v, want ErrInvalidDSN", err)
	}
	if d.cli != nil {
		t.Error("Init 失败后不应留下 client")
	}
}

func TestInit_PingFailure(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "memcache://localhost:1"})

	if err := d.Init(context.Background()); err == nil {
		t.Skip("localhost:1 竟然连通了，跳过")
	}
	if d.cli != nil {
		t.Error("Ping 失败后 cli 不应该被赋值")
	}
}

func TestShutdown_NoClient(t *testing.T) {
	d := newTestDB(t, Config{})

	if err := d.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestServersFor(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    []string
		wantErr error
	}{
		{name: "单 server", dsn: "memcache://localhost:11211", want: []string{"localhost:11211"}},
		{name: "多 server", dsn: "memcache://host1:11211,host2:11211", want: []string{"host1:11211", "host2:11211"}},
		{name: "带空格", dsn: "memcache://host1:11211, host2:11211", want: []string{"host1:11211", "host2:11211"}},
		{name: "无 scheme", dsn: "localhost:11211", wantErr: ErrInvalidDSN},
		{name: "错误 scheme", dsn: "redis://localhost:6379", wantErr: ErrInvalidDSN},
		{name: "仅 scheme", dsn: "memcache://", wantErr: ErrInvalidDSN},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			servers, err := serversFor(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("serversFor(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("serversFor(%q) unexpected err: %v", tt.dsn, err)
			}
			if len(servers) != len(tt.want) {
				t.Fatalf("serversFor(%q) = %v, want %v", tt.dsn, servers, tt.want)
			}
			for i := range servers {
				if servers[i] != tt.want[i] {
					t.Errorf("serversFor(%q)[%d] = %q, want %q", tt.dsn, i, servers[i], tt.want[i])
				}
			}
		})
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	servers, err := serversFor("memcache://localhost:11211")
	if err != nil {
		t.Errorf("默认 DSN 解析失败: %v", err)
	}
	if len(servers) != 1 || servers[0] != "localhost:11211" {
		t.Errorf("默认 DSN 解析结果 = %v, want [localhost:11211]", servers)
	}
}
