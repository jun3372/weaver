package nats

import (
	"context"
	"time"

	"github.com/jun3372/weaver"
	"github.com/nats-io/nats.go"
)

type T interface {
	Client(ctx context.Context) (*nats.Conn, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"nats"`
	conn                      *nats.Conn
}

func (d *db) Client(_ context.Context) (*nats.Conn, error) {
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
		d.Logger(ctx).Info("nats 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("nats 组件初始化开始", "config", conf)
	// nats.Connect 内部完成 PING/PONG 握手，多 server 用逗号分隔
	conn, err := nats.Connect(conf.DSN,
		nats.Timeout(5*time.Second),
		nats.RetryOnFailedConnect(false),
		nats.Name("weaver"),
	)
	if err != nil {
		d.Logger(ctx).Error("nats 组件初始化失败", "error", err)
		return err
	}

	if err := conn.FlushTimeout(5 * time.Second); err != nil {
		conn.Close()
		d.Logger(ctx).Error("nats 组件探活失败", "error", err)
		return err
	}

	d.conn = conn
	d.Logger(ctx).Debug("nats 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	if d.conn == nil {
		return nil
	}

	// Drain 优先于 Close：冲刷残留订阅后再退出
	return d.conn.Drain()
}
