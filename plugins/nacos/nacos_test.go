package nacos

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"unsafe"

	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
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

func TestNamingClient_Disabled(t *testing.T) {
	d := newTestDB(t, Config{Enabled: false})

	_, err := d.NamingClient(context.Background())
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("NamingClient err = %v, want ErrNotEnabled", err)
	}
}

func TestConfigClient_Disabled(t *testing.T) {
	d := newTestDB(t, Config{Enabled: false})

	_, err := d.ConfigClient(context.Background())
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ConfigClient err = %v, want ErrNotEnabled", err)
	}
}

func TestNamingClient_NotReady(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true})

	_, err := d.NamingClient(context.Background())
	if !errors.Is(err, ErrNotReady) {
		t.Errorf("NamingClient err = %v, want ErrNotReady", err)
	}
}

func TestConfigClient_NotReady(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true})

	_, err := d.ConfigClient(context.Background())
	if !errors.Is(err, ErrNotReady) {
		t.Errorf("ConfigClient err = %v, want ErrNotReady", err)
	}
}

func TestInit_Disabled(t *testing.T) {
	d := newTestDB(t, Config{Enabled: false})

	if err := d.Init(context.Background()); err != nil {
		t.Fatalf("Init err = %v, want nil", err)
	}
	if d.naming != nil || d.config != nil {
		t.Error("Init 应该不创建 client")
	}
}

func TestInit_InvalidDSN(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "http://localhost:8848"})

	if err := d.Init(context.Background()); !errors.Is(err, ErrInvalidScheme) {
		t.Fatalf("Init err = %v, want ErrInvalidScheme", err)
	}
	if d.naming != nil || d.config != nil {
		t.Error("Init 失败后不应留下 client")
	}
}

func TestInit_PingFailure(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "nacos://localhost:1"})

	if err := d.Init(context.Background()); err == nil {
		t.Skip("localhost:1 竟然连通了，跳过")
	}
	if d.naming != nil || d.config != nil {
		t.Error("探活失败后 client 不应该被赋值")
	}
}

func TestShutdown_NoClient(t *testing.T) {
	d := newTestDB(t, Config{})

	if err := d.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestParseDSN(t *testing.T) {
	tests := []struct {
		name       string
		dsn        string
		wantIP     string
		wantPort   uint64
		wantUser   string
		wantPass   string
		wantNS     string
		wantServer int
		wantErr    error
	}{
		{name: "最简", dsn: "nacos://localhost:8848", wantIP: "localhost", wantPort: 8848, wantServer: 1},
		{name: "缺省端口", dsn: "nacos://localhost", wantIP: "localhost", wantPort: 8848, wantServer: 1},
		{name: "多 server", dsn: "nacos://host1:8848,host2:8848", wantIP: "host1", wantPort: 8848, wantServer: 2},
		{name: "带 userinfo", dsn: "nacos://nacos:nacos@localhost:8848", wantIP: "localhost", wantPort: 8848, wantUser: "nacos", wantPass: "nacos", wantServer: 1},
		{name: "带 namespace", dsn: "nacos://localhost:8848?namespaceId=dev", wantIP: "localhost", wantPort: 8848, wantNS: "dev", wantServer: 1},
		{name: "无 scheme", dsn: "localhost:8848", wantErr: ErrInvalidDSN},
		{name: "错误 scheme", dsn: "http://localhost:8848", wantErr: ErrInvalidScheme},
		{name: "端口非法", dsn: "nacos://localhost:abc", wantErr: ErrInvalidDSN},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			servers, ns, user, pass, err := parseDSN(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("parseDSN(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDSN(%q) unexpected err: %v", tt.dsn, err)
			}
			if len(servers) != tt.wantServer {
				t.Fatalf("parseDSN(%q) servers = %d 个, want %d 个", tt.dsn, len(servers), tt.wantServer)
			}
			if servers[0].IpAddr != tt.wantIP || servers[0].Port != tt.wantPort {
				t.Errorf("parseDSN(%q) server = %s:%d, want %s:%d", tt.dsn, servers[0].IpAddr, servers[0].Port, tt.wantIP, tt.wantPort)
			}
			if user != tt.wantUser || pass != tt.wantPass || ns != tt.wantNS {
				t.Errorf("parseDSN(%q) user/pass/ns = %q/%q/%q, want %q/%q/%q", tt.dsn, user, pass, ns, tt.wantUser, tt.wantPass, tt.wantNS)
			}
		})
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	servers, _, _, _, err := parseDSN("nacos://localhost:8848")
	if err != nil {
		t.Errorf("默认 DSN 解析失败: %v", err)
	}
	if len(servers) != 1 || servers[0].IpAddr != "localhost" || servers[0].Port != 8848 {
		t.Errorf("默认 DSN 解析结果异常: %+v", servers)
	}
}

func TestDefaultGroup(t *testing.T) {
	if constant.DEFAULT_GROUP != "DEFAULT_GROUP" {
		t.Errorf("constant.DEFAULT_GROUP = %q, want DEFAULT_GROUP", constant.DEFAULT_GROUP)
	}
}
