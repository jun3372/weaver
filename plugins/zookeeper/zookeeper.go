package zookeeper

import (
	"context"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
	"github.com/jun3372/weaver"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (*zk.Conn, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"zookeeper"`
	conn                      *zk.Conn
}

func (d *db) Client(_ context.Context) (*zk.Conn, error) {
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
		d.Logger(ctx).Info("zookeeper 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("zookeeper 组件初始化开始", "config", conf)
	hosts, err := hostsFor(conf.DSN)
	if err != nil {
		return err
	}

	// zk.Connect 内部按 session timeout 重试建连
	conn, _, err := zk.Connect(hosts, 5*time.Second)
	if err != nil {
		d.Logger(ctx).Error("zookeeper 组件初始化失败", "error", err)
		return err
	}

	if conn.State() != zk.StateConnected {
		conn.Close()
		err = errors.WithMessage(ErrNotReady, "未进入 connected 状态")
		d.Logger(ctx).Error("zookeeper 组件探活失败", "error", err)
		return err
	}

	d.conn = conn
	d.Logger(ctx).Debug("zookeeper 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	if d.conn == nil {
		return nil
	}

	d.conn.Close()
	return nil
}

// hostsFor 兼容 `zk://host:2181` 与 `zk://host1:2181,host2:2181` 两种 DSN 形式。
func hostsFor(dsn string) ([]string, error) {
	scheme, rest, ok := strings.Cut(dsn, ":")
	if !ok || scheme != "zk" {
		return nil, ErrInvalidDSN
	}
	rest = strings.TrimPrefix(rest, "//")

	var hosts []string
	for _, h := range strings.Split(rest, ",") {
		if h = strings.TrimSpace(h); h != "" {
			hosts = append(hosts, h)
		}
	}
	if len(hosts) == 0 {
		return nil, errors.WithMessage(ErrInvalidDSN, "无可用 host")
	}
	return hosts, nil
}
