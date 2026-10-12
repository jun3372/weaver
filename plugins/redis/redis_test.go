package redis

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"unsafe"

	"github.com/pkg/errors"
	goredis "github.com/redis/go-redis/v9"
)

func newTestDB(t *testing.T, conf Config) *db {
	t.Helper()
	r := &db{}
	r.SetConfig(conf)
	// setLogger 未导出且定义在 weaver 包，测试里经反射注入
	f := reflect.ValueOf(r).Elem().FieldByName("Implements").FieldByName("logger")
	reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().
		Set(reflect.ValueOf(slog.Default()))
	return r
}

func TestClient_Disabled(t *testing.T) {
	r := newTestDB(t, Config{Enabled: false})

	for name, fn := range map[string]func(context.Context) (*goredis.Client, error){
		"Client": r.Client,
		"DB":     r.DB,
	} {
		_, err := fn(context.Background())
		if !errors.Is(err, ErrNotEnabled) {
			t.Errorf("%s err = %v, want ErrNotEnabled", name, err)
		}
	}
}

func TestClient_NotReady(t *testing.T) {
	r := newTestDB(t, Config{Enabled: true})

	_, err := r.Client(context.Background())
	if !errors.Is(err, ErrNotReady) {
		t.Errorf("Client err = %v, want ErrNotReady", err)
	}
}

func TestInit_Disabled(t *testing.T) {
	r := newTestDB(t, Config{Enabled: false})

	if err := r.Init(context.Background()); err != nil {
		t.Fatalf("Init err = %v, want nil", err)
	}
	if r.rdb != nil {
		t.Error("Init 应该不创建 client")
	}
}

func TestInit_InvalidDSN(t *testing.T) {
	r := newTestDB(t, Config{Enabled: true, DSN: "://bad"})

	if err := r.Init(context.Background()); err == nil {
		t.Fatal("Init err = nil, want DSN 解析错误")
	}
	if r.rdb != nil {
		t.Error("Init 失败后不应留下 client")
	}
}

func TestInit_PingFailure(t *testing.T) {
	r := newTestDB(t, Config{Enabled: true, DSN: "redis://localhost:1/0?dial_timeout=200ms&max_retries=0"})

	if err := r.Init(context.Background()); err == nil {
		t.Skip("localhost:1 竟然连通了，跳过")
	}
	if r.rdb != nil {
		t.Error("Ping 失败后 rdb 应该被 Close 并置 nil")
	}
}

func TestShutdown_NoClient(t *testing.T) {
	r := newTestDB(t, Config{})

	if err := r.Shutdown(); err != nil {
		t.Errorf("Shutdown err = %v, want nil", err)
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	if _, err := goredis.ParseURL("redis://localhost:6379/0"); err != nil {
		t.Errorf("默认 DSN 解析失败: %v", err)
	}
}
