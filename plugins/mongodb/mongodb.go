package mongodb

import (
	"context"
	"time"

	"github.com/jun3372/weaver"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type T interface {
	Client(ctx context.Context) (*mongo.Client, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"mongodb"`
	cli                       *mongo.Client
}

func (d *db) Client(_ context.Context) (*mongo.Client, error) {
	if !d.Config().Enabled {
		return nil, ErrNotEnabled
	}

	if d.cli == nil {
		return nil, ErrNotReady
	}

	return d.cli, nil
}

func (d *db) Init(ctx context.Context) error {
	conf := d.Config()
	if !conf.Enabled {
		d.Logger(ctx).Info("mongodb 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("mongodb 组件初始化开始", "config", conf)
	cli, err := mongo.Connect(options.Client().ApplyURI(conf.DSN))
	if err != nil {
		d.Logger(ctx).Error("mongodb 组件初始化失败", "error", err)
		return err
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cli.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = cli.Disconnect(pingCtx)
		d.Logger(ctx).Error("mongodb 组件 Ping 失败", "error", err)
		return err
	}

	d.cli = cli
	d.Logger(ctx).Debug("mongodb 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	if d.cli == nil {
		return nil
	}

	return d.cli.Disconnect(context.Background())
}
