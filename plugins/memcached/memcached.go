package memcached

import (
	"context"
	"strings"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/jun3372/weaver"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (*memcache.Client, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"memcached"`
	cli                       *memcache.Client
}

func (d *db) Client(_ context.Context) (*memcache.Client, error) {
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
		d.Logger(ctx).Info("memcached 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("memcached 组件初始化开始", "config", conf)
	servers, err := serversFor(conf.DSN)
	if err != nil {
		return err
	}

	cli := memcache.New(servers...)
	if err := cli.Ping(); err != nil {
		d.Logger(ctx).Error("memcached 组件探活失败", "error", err)
		return err
	}

	d.cli = cli
	d.Logger(ctx).Debug("memcached 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	// gomemcache client 内部管理连接，无需显式关闭
	return nil
}

// serversFor 兼容 `memcache://host:11211` 与 `memcache://host1:11211,host2:11211`
// 两种 DSN 形式。
func serversFor(dsn string) ([]string, error) {
	scheme, rest, ok := strings.Cut(dsn, ":")
	if !ok || scheme != "memcache" {
		return nil, ErrInvalidDSN
	}
	rest = strings.TrimPrefix(rest, "//")

	var servers []string
	for _, s := range strings.Split(rest, ",") {
		if s = strings.TrimSpace(s); s != "" {
			servers = append(servers, s)
		}
	}
	if len(servers) == 0 {
		return nil, errors.WithMessage(ErrInvalidDSN, "无可用 server")
	}
	return servers, nil
}
