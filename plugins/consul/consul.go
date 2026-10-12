package consul

import (
	"context"
	"net/url"

	"github.com/hashicorp/consul/api"
	"github.com/jun3372/weaver"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (*api.Client, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"consul"`
	cli                       *api.Client
}

func (d *db) Client(_ context.Context) (*api.Client, error) {
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
		d.Logger(ctx).Info("consul 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("consul 组件初始化开始", "config", conf)
	cfg, err := configFor(conf.DSN)
	if err != nil {
		return err
	}

	cli, err := api.NewClient(cfg)
	if err != nil {
		d.Logger(ctx).Error("consul 组件初始化失败", "error", err)
		return err
	}

	if _, err := cli.Status().Leader(); err != nil {
		d.Logger(ctx).Error("consul 组件探活失败", "error", err)
		return err
	}

	d.cli = cli
	d.Logger(ctx).Debug("consul 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	// HTTP client 无需显式关闭
	return nil
}

// configFor 兼容 `http://127.0.0.1:8500` 与 `http://user:pass@127.0.0.1:8500`
// （userinfo 转 Basic Auth）两种 DSN 形式。
func configFor(dsn string) (*api.Config, error) {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.WithMessage(ErrInvalidDSN, "dsn: "+dsn)
	}

	cfg := api.DefaultConfig()
	cfg.Address = u.Host
	cfg.Scheme = u.Scheme
	if u.User != nil {
		pass, _ := u.User.Password()
		cfg.HttpAuth = &api.HttpBasicAuth{Username: u.User.Username(), Password: pass}
	}
	return cfg, nil
}
