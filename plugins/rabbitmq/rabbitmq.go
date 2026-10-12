package rabbitmq

import (
	"context"

	"github.com/jun3372/weaver"
	amqp "github.com/rabbitmq/amqp091-go"
)

type T interface {
	Client(ctx context.Context) (*amqp.Connection, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"rabbitmq"`
	conn                      *amqp.Connection
}

func (d *db) Client(_ context.Context) (*amqp.Connection, error) {
	if !d.Config().Enabled {
		return nil, ErrNotEnabled
	}

	if d.conn == nil {
		return nil, ErrNotReady
	}

	return d.conn, nil
}

func (d *db) Init(ctx context.Context) error {
	conf := d.Config()
	if !conf.Enabled {
		d.Logger(ctx).Info("rabbitmq 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("rabbitmq 组件初始化开始", "config", conf)
	conn, err := amqp.Dial(conf.DSN)
	if err != nil {
		d.Logger(ctx).Error("rabbitmq 组件初始化失败", "error", err)
		return err
	}

	// 探活：Channel 建立成功
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		d.Logger(ctx).Error("rabbitmq 组件 Channel 建立失败", "error", err)
		return err
	}
	_ = ch.Close()

	d.conn = conn
	d.Logger(ctx).Debug("rabbitmq 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	if d.conn == nil {
		return nil
	}

	return d.conn.Close()
}
