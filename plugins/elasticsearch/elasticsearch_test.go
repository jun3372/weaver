package elasticsearch

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
	d := newTestDB(t, Config{Enabled: true, DSN: "localhost:9200"})

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
		t.Error("Ping 失败后 cli 不应该被赋值")
	}
}

func TestShutdown_NoClient(t *testing.T) {
	d := newTestDB(t, Config{})

	if err := d.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestAddressesFor(t *testing.T) {
	tests := []struct {
		name      string
		dsn       string
		wantAddrs []string
		wantUser  string
		wantPass  string
		wantErr   error
	}{
		{name: "单地址", dsn: "http://localhost:9200", wantAddrs: []string{"http://localhost:9200"}},
		{name: "多地址", dsn: "http://host1:9200,http://host2:9200", wantAddrs: []string{"http://host1:9200", "http://host2:9200"}},
		{name: "带空格", dsn: "http://host1:9200, http://host2:9200", wantAddrs: []string{"http://host1:9200", "http://host2:9200"}},
		{name: "带 userinfo", dsn: "http://elastic:secret@localhost:9200", wantAddrs: []string{"http://localhost:9200"}, wantUser: "elastic", wantPass: "secret"},
		{name: "https", dsn: "https://localhost:9200", wantAddrs: []string{"https://localhost:9200"}},
		{name: "无 scheme", dsn: "localhost:9200", wantErr: ErrInvalidDSN},
		{name: "错误 scheme", dsn: "ftp://localhost:9200", wantErr: ErrInvalidDSN},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs, user, pass, err := addressesFor(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("addressesFor(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("addressesFor(%q) unexpected err: %v", tt.dsn, err)
			}
			if len(addrs) != len(tt.wantAddrs) {
				t.Fatalf("addressesFor(%q) addrs = %v, want %v", tt.dsn, addrs, tt.wantAddrs)
			}
			for i := range addrs {
				if addrs[i] != tt.wantAddrs[i] {
					t.Errorf("addressesFor(%q)[%d] = %q, want %q", tt.dsn, i, addrs[i], tt.wantAddrs[i])
				}
			}
			if user != tt.wantUser || pass != tt.wantPass {
				t.Errorf("addressesFor(%q) user/pass = %q/%q, want %q/%q", tt.dsn, user, pass, tt.wantUser, tt.wantPass)
			}
		})
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	addrs, _, _, err := addressesFor("http://localhost:9200")
	if err != nil {
		t.Errorf("默认 DSN 解析失败: %v", err)
	}
	if len(addrs) != 1 || addrs[0] != "http://localhost:9200" {
		t.Errorf("默认 DSN 解析结果 = %v, want [http://localhost:9200]", addrs)
	}
}
