package zookeeper

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
	if d.conn != nil {
		t.Error("Init 应该不创建连接")
	}
}

func TestInit_InvalidDSN(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "localhost:2181"})

	if err := d.Init(context.Background()); !errors.Is(err, ErrInvalidDSN) {
		t.Fatalf("Init err = %v, want ErrInvalidDSN", err)
	}
	if d.conn != nil {
		t.Error("Init 失败后不应留下连接")
	}
}

func TestInit_PingFailure(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "zk://localhost:1"})

	if err := d.Init(context.Background()); err == nil {
		t.Skip("localhost:1 竟然连通了，跳过")
	}
	if d.conn != nil {
		t.Error("连接失败后 conn 不应该被赋值")
	}
}

func TestShutdown_NoConn(t *testing.T) {
	d := newTestDB(t, Config{})

	if err := d.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestHostsFor(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    []string
		wantErr error
	}{
		{name: "单 host", dsn: "zk://localhost:2181", want: []string{"localhost:2181"}},
		{name: "多 host", dsn: "zk://host1:2181,host2:2181", want: []string{"host1:2181", "host2:2181"}},
		{name: "带空格", dsn: "zk://host1:2181, host2:2181", want: []string{"host1:2181", "host2:2181"}},
		{name: "无 scheme", dsn: "localhost:2181", wantErr: ErrInvalidDSN},
		{name: "错误 scheme", dsn: "kafka://localhost:9092", wantErr: ErrInvalidDSN},
		{name: "仅 scheme", dsn: "zk://", wantErr: ErrInvalidDSN},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hosts, err := hostsFor(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("hostsFor(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("hostsFor(%q) unexpected err: %v", tt.dsn, err)
			}
			if len(hosts) != len(tt.want) {
				t.Fatalf("hostsFor(%q) = %v, want %v", tt.dsn, hosts, tt.want)
			}
			for i := range hosts {
				if hosts[i] != tt.want[i] {
					t.Errorf("hostsFor(%q)[%d] = %q, want %q", tt.dsn, i, hosts[i], tt.want[i])
				}
			}
		})
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	hosts, err := hostsFor("zk://localhost:2181")
	if err != nil {
		t.Errorf("默认 DSN 解析失败: %v", err)
	}
	if len(hosts) != 1 || hosts[0] != "localhost:2181" {
		t.Errorf("默认 DSN 解析结果 = %v, want [localhost:2181]", hosts)
	}
}
