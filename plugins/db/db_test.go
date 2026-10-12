package db

import (
	"testing"

	"github.com/pkg/errors"
)

func TestDialectorFor(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr error
	}{
		{name: "默认配置", dsn: "file:data/data.db?cache=shared&mode=rwc&_busy_timeout=5000&_journal_mode=WAL"},
		{name: "file 无斜杠", dsn: "file:test.db"},
		{name: "sqlite 双斜杠", dsn: "sqlite://file:test.db?mode=memory"},
		{name: "sqlite 双斜杠裸路径", dsn: "sqlite://test.db"},
		{name: "sqlite 单冒号", dsn: "sqlite:test.db"},
		{name: "sqlite3 双斜杠", dsn: "sqlite3://:memory:"},
		{name: "mysql", dsn: "mysql://user:pass@tcp(127.0.0.1:3306)/db"},
		{name: "postgres", dsn: "postgres://user:pass@localhost:5432/db?sslmode=disable"},
		{name: "postgresql 别名", dsn: "postgresql://user:pass@localhost:5432/db"},
		{name: "clickhouse", dsn: "clickhouse://localhost:9000?username=gorm&password=gorm&database=gorm"},
		{name: "clickhouse 单冒号", dsn: "clickhouse:localhost:9000?database=gorm"},
		{name: "无 scheme", dsn: "data/data.db", wantErr: ErrInvalidDSN},
		{name: "空 DSN", dsn: "", wantErr: ErrInvalidDSN},
		{name: "未知 scheme", dsn: "oracle://localhost/db", wantErr: ErrInvalidScheme},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dial, err := dialectorFor(tt.dsn)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("dialectorFor(%q) err = %v, want %v", tt.dsn, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("dialectorFor(%q) unexpected err: %v", tt.dsn, err)
			}
			if dial == nil {
				t.Fatalf("dialectorFor(%q) = nil, want dialector", tt.dsn)
			}
			t.Logf("dialectorFor(%q) -> %T", tt.dsn, dial)
		})
	}
}
