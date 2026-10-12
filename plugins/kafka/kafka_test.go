package kafka

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"unsafe"

	"github.com/IBM/sarama"
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
	d := newTestDB(t, Config{Enabled: true, DSN: "localhost:9092"})

	if err := d.Init(context.Background()); !errors.Is(err, ErrInvalidDSN) {
		t.Fatalf("Init err = %v, want ErrInvalidDSN", err)
	}
	if d.cli != nil {
		t.Error("Init 失败后不应留下 client")
	}
}

func TestInit_PingFailure(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "kafka://localhost:1"})

	if err := d.Init(context.Background()); err == nil {
		t.Skip("localhost:1 竟然连通了，跳过")
	}
	if d.cli != nil {
		t.Error("Init 失败后不应留下 client")
	}
}

func TestShutdown_NoClient(t *testing.T) {
	d := newTestDB(t, Config{})

	if err := d.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestBrokersFor(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    []string
		wantErr error
	}{
		{name: "单 broker", dsn: "kafka://localhost:9092", want: []string{"localhost:9092"}},
		{name: "多 broker", dsn: "kafka://host1:9092,host2:9092", want: []string{"host1:9092", "host2:9092"}},
		{name: "带空格", dsn: "kafka://host1:9092, host2:9092", want: []string{"host1:9092", "host2:9092"}},
		{name: "无 scheme", dsn: "localhost:9092", wantErr: ErrInvalidDSN},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
		{name: "错误 scheme", dsn: "redis://localhost:6379", wantErr: ErrInvalidDSN},
		{name: "仅 scheme", dsn: "kafka://", wantErr: ErrInvalidDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokers, err := brokersFor(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("brokersFor(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("brokersFor(%q) unexpected err: %v", tt.dsn, err)
			}
			if len(brokers) != len(tt.want) {
				t.Fatalf("brokersFor(%q) = %v, want %v", tt.dsn, brokers, tt.want)
			}
			for i := range brokers {
				if brokers[i] != tt.want[i] {
					t.Errorf("brokersFor(%q)[%d] = %q, want %q", tt.dsn, i, brokers[i], tt.want[i])
				}
			}
		})
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	brokers, err := brokersFor("kafka://localhost:9092")
	if err != nil {
		t.Errorf("默认 DSN 解析失败: %v", err)
	}
	if len(brokers) != 1 || brokers[0] != "localhost:9092" {
		t.Errorf("默认 DSN 解析结果 = %v, want [localhost:9092]", brokers)
	}
}

func TestDefaultVersion(t *testing.T) {
	cfg := sarama.NewConfig()
	if cfg.Version != sarama.DefaultVersion {
		t.Errorf("cfg.Version = %v, want DefaultVersion", cfg.Version)
	}
}
