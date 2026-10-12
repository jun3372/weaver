package s3

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
	for _, dsn := range []string{
		"localhost:9000",                         // 无 scheme
		"s3://localhost:9000",                    // 缺 AK/SK
		"http://ak:sk@localhost:9000",            // 错误 scheme
		"s3://ak:sk@localhost:9000?secure=maybe", // secure 非法
	} {
		d := newTestDB(t, Config{Enabled: true, DSN: dsn})
		if err := d.Init(context.Background()); !errors.Is(err, ErrInvalidDSN) && !errors.Is(err, ErrInvalidScheme) {
			t.Fatalf("Init(%q) err = %v, want ErrInvalidDSN/ErrInvalidScheme", dsn, err)
		}
		if d.cli != nil {
			t.Errorf("Init(%q) 失败后不应留下 client", dsn)
		}
	}
}

func TestInit_BucketProbeFailure(t *testing.T) {
	d := newTestDB(t, Config{Enabled: true, DSN: "s3://ak:sk@localhost:1/bucket?secure=false"})

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

func TestParseDSN(t *testing.T) {
	tests := []struct {
		name         string
		dsn          string
		wantEndpoint string
		wantAK       string
		wantSK       string
		wantBucket   string
		wantSecure   bool
		wantErr      error
	}{
		{name: "最简", dsn: "s3://ak:sk@localhost:9000", wantEndpoint: "localhost:9000", wantAK: "ak", wantSK: "sk"},
		{name: "带 bucket", dsn: "s3://ak:sk@localhost:9000/mybucket", wantEndpoint: "localhost:9000", wantAK: "ak", wantSK: "sk", wantBucket: "mybucket"},
		{name: "secure=true", dsn: "s3://ak:sk@s3.amazonaws.com?secure=true", wantEndpoint: "s3.amazonaws.com", wantAK: "ak", wantSK: "sk", wantSecure: true},
		{name: "缺 secret", dsn: "s3://ak@localhost:9000", wantEndpoint: "localhost:9000", wantAK: "ak", wantSK: ""},
		{name: "无 scheme", dsn: "localhost:9000", wantErr: ErrInvalidDSN},
		{name: "错误 scheme", dsn: "redis://ak:sk@localhost:6379", wantErr: ErrInvalidScheme},
		{name: "缺 userinfo", dsn: "s3://localhost:9000", wantErr: ErrInvalidDSN},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, ak, sk, bucket, secure, err := parseDSN(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("parseDSN(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDSN(%q) unexpected err: %v", tt.dsn, err)
			}
			if endpoint != tt.wantEndpoint || ak != tt.wantAK || sk != tt.wantSK || bucket != tt.wantBucket || secure != tt.wantSecure {
				t.Errorf("parseDSN(%q) = %q/%q/%q/%q/%v, want %q/%q/%q/%q/%v",
					tt.dsn, endpoint, ak, sk, bucket, secure, tt.wantEndpoint, tt.wantAK, tt.wantSK, tt.wantBucket, tt.wantSecure)
			}
		})
	}
}

func TestDefaultDSNParseable(t *testing.T) {
	endpoint, ak, sk, bucket, secure, err := parseDSN("s3://minioadmin:minioadmin@localhost:9000?secure=false")
	if err != nil {
		t.Errorf("默认 DSN 解析失败: %v", err)
	}
	if endpoint != "localhost:9000" || ak != "minioadmin" || sk != "minioadmin" || bucket != "" || secure {
		t.Errorf("默认 DSN 解析结果 = %q/%q/%q/%q/%v", endpoint, ak, sk, bucket, secure)
	}
}
