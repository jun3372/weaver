package redis

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/jun3372/weaver"
)

type T interface {
	DB(context.Context) (*redis.Client, error)
	Client(context.Context) (*redis.Client, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `config:"redis"`
	rdb                       *redis.Client
}

func (r *db) DB(ctx context.Context) (*redis.Client, error) {
	return r.Client(ctx)
}

func (r *db) Client(_ context.Context) (*redis.Client, error) {
	if !r.Config().Enabled {
		return nil, ErrNotEnabled
	}

	if r.rdb == nil {
		return nil, ErrNotReady
	}

	return r.rdb, nil
}

func (r *db) Init(ctx context.Context) error {
	conf := r.Config()
	if !conf.Enabled {
		r.Logger(ctx).Info("redis 组件未启用")
		return nil
	}

	opts, err := redis.ParseURL(conf.DSN)
	if err != nil {
		return err
	}

	r.rdb = redis.NewClient(opts)
	if err := r.rdb.Ping(ctx).Err(); err != nil {
		_ = r.rdb.Close()
		r.rdb = nil
		return err
	}
	return nil
}

func (r *db) Shutdown() error {
	if r.rdb == nil {
		return nil
	}

	return r.rdb.Close()
}
