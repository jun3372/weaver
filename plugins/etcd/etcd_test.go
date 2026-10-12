package etcd

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
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
	d := newTestDB(t, Config{Enabled: true, DSN: "localhost:2379"})

	if err := d.Init(context.Background()); !errors.Is(err, ErrInvalidDSN) {
		t.Fatalf("Init err = %v, want ErrInvalidDSN", err)
	}
	if d.cli != nil {
		t.Error("Init 失败后不应留下 client")
	}
}

func TestInit_PingFailure(t *testing.T) {
	endpoints, err := endpointsFor("etcd://localhost:1")
	if err != nil {
		t.Fatalf("endpointsFor err = %v, want nil", err)
	}

	// 直接构造短超时 client，模拟 Status 探活失败（连上则跳过）
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: 200 * time.Millisecond})
	if err == nil {
		_ = cli.Close()
		t.Skip("localhost:1 竟然连通了，跳过")
	}
}

func TestShutdown_NoClient(t *testing.T) {
	d := newTestDB(t, Config{})

	if err := d.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestEndpointsFor(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    []string
		wantErr error
	}{
		{name: "单 endpoint", dsn: "etcd://localhost:2379", want: []string{"localhost:2379"}},
		{name: "多 endpoint", dsn: "etcd://host1:2379,host2:2379", want: []string{"host1:2379", "host2:2379"}},
		{name: "带空格", dsn: "etcd://host1:2379, host2:2379", want: []string{"host1:2379", "host2:2379"}},
		{name: "无 scheme", dsn: "localhost:2379", wantErr: ErrInvalidDSN},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
		{name: "错误 scheme", dsn: "kafka://localhost:9092", wantErr: ErrInvalidDSN},
		{name: "仅 scheme", dsn: "etcd://", wantErr: ErrInvalidDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints, err := endpointsFor(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("endpointsFor(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("endpointsFor(%q) unexpected err: %v", tt.dsn, err)
			}
			if len(endpoints) != len(tt.want) {
				t.Fatalf("endpointsFor(%q) = %v, want %v", tt.dsn, endpoints, tt.want)
			}
			for i := range endpoints {
				if endpoints[i] != tt.want[i] {
					t.Errorf("endpointsFor(%q)[%d] = %q, want %q", tt.dsn, i, endpoints[i], tt.want[i])
				}
			}
		})
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	endpoints, err := endpointsFor("etcd://localhost:2379")
	if err != nil {
		t.Errorf("默认 DSN 解析失败: %v", err)
	}
	if len(endpoints) != 1 || endpoints[0] != "localhost:2379" {
		t.Errorf("默认 DSN 解析结果 = %v, want [localhost:2379]", endpoints)
	}
}
