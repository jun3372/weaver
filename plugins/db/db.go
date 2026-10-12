package db

import (
	"context"
	"database/sql"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/jun3372/weaver"
	"github.com/pkg/errors"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type T interface {
	DB(context.Context) (*gorm.DB, error)
	WithContext(context.Context) (*gorm.DB, error)
	Transaction(context.Context, func(tx *gorm.DB) error, ...*sql.TxOptions) error
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"db"`
	db                        *gorm.DB
}

func (d *db) WithContext(ctx context.Context) (*gorm.DB, error) {
	if !d.Config().Enabled {
		return nil, ErrNotEnabled
	}

	if d.db == nil {
		return nil, ErrNotReady
	}

	return d.db.WithContext(ctx), nil
}

func (d *db) DB(ctx context.Context) (*gorm.DB, error) {
	return d.WithContext(ctx)
}

func (d *db) Transaction(ctx context.Context, fn func(tx *gorm.DB) error, opts ...*sql.TxOptions) error {
	gdb, err := d.WithContext(ctx)
	if err != nil {
		return err
	}
	return gdb.Transaction(fn, opts...)
}

func (d *db) Shutdown() error {
	if d.db == nil {
		return nil
	}

	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (d *db) Init(ctx context.Context) error {
	conf := d.Config()
	if !conf.Enabled {
		d.Logger(ctx).Info("db 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("db 组件初始化开始", "config", conf)
	dialector, err := d.dialector()
	if err != nil {
		return err
	}

	d.Logger(ctx).Debug("db 组件初始化中", "dialector", dialector)
	opts := conf.GormConfig
	if opts == nil {
		opts = &gorm.Config{}
	}
	if opts.Logger == nil {
		opts.Logger = newSlogLogger(d.Logger)
	}

	db, err := gorm.Open(dialector, opts)
	if err != nil {
		d.Logger(ctx).Error("db 组件初始化失败", "error", err)
		return err
	}

	d.db = db
	d.Logger(ctx).Debug("db 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) dialector() (gorm.Dialector, error) {
	return dialectorFor(d.Config().DSN)
}

// 兼容 `scheme://...` 与 `scheme:...` 两种 DSN 形式，
// 如 sqlite://file:test.db、sqlite:test.db、file:test.db?opts、mysql://...。
func dialectorFor(dsn string) (gorm.Dialector, error) {
	scheme, rest, ok := strings.Cut(dsn, ":")
	if !ok {
		return nil, ErrInvalidDSN
	}
	rest = strings.TrimPrefix(rest, "//")

	switch scheme {
	case "sqlite3", "sqlite":
		return sqlite.Open(rest), nil
	case "file":
		return sqlite.Open("file:" + rest), nil
	case "mysql":
		return mysql.Open(rest), nil
	case "postgres", "postgresql":
		return postgres.Open(rest), nil
	case "clickhouse":
		return clickhouse.Open(rest), nil
	default:
		return nil, errors.WithMessage(ErrInvalidScheme, "scheme: "+scheme)
	}
}
