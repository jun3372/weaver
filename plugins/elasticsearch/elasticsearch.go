package elasticsearch

import (
	"context"
	"net/url"
	"strings"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/jun3372/weaver"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (*elasticsearch.Client, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"elasticsearch"`
	cli                       *elasticsearch.Client
}

func (d *db) Client(_ context.Context) (*elasticsearch.Client, error) {
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
		d.Logger(ctx).Info("elasticsearch 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("elasticsearch 组件初始化开始", "config", conf)
	addrs, user, pass, err := addressesFor(conf.DSN)
	if err != nil {
		return err
	}

	cli, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: addrs,
		Username:  user,
		Password:  pass,
	})
	if err != nil {
		d.Logger(ctx).Error("elasticsearch 组件初始化失败", "error", err)
		return err
	}

	res, err := cli.Ping()
	if err != nil {
		d.Logger(ctx).Error("elasticsearch 组件 Ping 失败", "error", err)
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		err = errors.Errorf("elasticsearch ping 响应异常: %s", res.Status())
		d.Logger(ctx).Error("elasticsearch 组件 Ping 失败", "error", err)
		return err
	}

	d.cli = cli
	d.Logger(ctx).Debug("elasticsearch 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	// 官方 client 内部管理连接，无需显式关闭
	return nil
}

// addressesFor 兼容 `http://host:9200`、`http://host1:9200,http://host2:9200` 与
// `http://user:pass@host:9200`（Basic Auth 取首个带 userinfo 的地址）三种 DSN 形式。
func addressesFor(dsn string) (addrs []string, user, pass string, err error) {
	for _, a := range strings.Split(dsn, ",") {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}

		u, perr := url.Parse(a)
		if perr != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, "", "", errors.WithMessage(ErrInvalidDSN, "address: "+a)
		}
		if u.User != nil && user == "" {
			user = u.User.Username()
			pass, _ = u.User.Password()
		}
		addrs = append(addrs, u.Scheme+"://"+u.Host)
	}

	if len(addrs) == 0 {
		return nil, "", "", errors.WithMessage(ErrInvalidDSN, "无可用地址")
	}
	return addrs, user, pass, nil
}
