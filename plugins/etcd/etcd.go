package etcd

import (
	"context"
	"strings"
	"time"

	"github.com/jun3372/weaver"
	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type T interface {
	Client(ctx context.Context) (*clientv3.Client, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"etcd"`
	cli                       *clientv3.Client
}

func (d *db) Client(_ context.Context) (*clientv3.Client, error) {
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
		d.Logger(ctx).Info("etcd 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("etcd 组件初始化开始", "config", conf)
	endpoints, err := endpointsFor(conf.DSN)
	if err != nil {
		return err
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		d.Logger(ctx).Error("etcd 组件初始化失败", "error", err)
		return err
	}

	stCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Status(stCtx, endpoints[0]); err != nil {
		_ = cli.Close()
		d.Logger(ctx).Error("etcd 组件 Status 探活失败", "error", err)
		return err
	}

	d.cli = cli
	d.Logger(ctx).Debug("etcd 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	if d.cli == nil {
		return nil
	}

	return d.cli.Close()
}

// endpointsFor 兼容 `etcd://host:2379` 与 `etcd://host1:2379,host2:2379` 两种 DSN 形式。
func endpointsFor(dsn string) ([]string, error) {
	scheme, rest, ok := strings.Cut(dsn, ":")
	if !ok || scheme != "etcd" {
		return nil, ErrInvalidDSN
	}
	rest = strings.TrimPrefix(rest, "//")

	var endpoints []string
	for _, e := range strings.Split(rest, ",") {
		if e = strings.TrimSpace(e); e != "" {
			endpoints = append(endpoints, e)
		}
	}
	if len(endpoints) == 0 {
		return nil, errors.WithMessage(ErrInvalidDSN, "无可用 endpoint")
	}
	return endpoints, nil
}
