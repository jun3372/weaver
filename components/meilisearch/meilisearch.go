package meilisearch

import (
	"context"
	"net/url"

	"github.com/jun3372/weaver"
	"github.com/meilisearch/meilisearch-go"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (meilisearch.ServiceManager, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"meilisearch"`
	cli                       meilisearch.ServiceManager
}

func (d *db) Client(_ context.Context) (meilisearch.ServiceManager, error) {
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
		d.Logger(ctx).Info("meilisearch 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("meilisearch 组件初始化开始", "host", conf.Host)
	host, err := hostFor(conf.Host)
	if err != nil {
		return err
	}

	var opts []meilisearch.Option
	if conf.APIKey != "" {
		opts = append(opts, meilisearch.WithAPIKey(conf.APIKey))
	}
	cli := meilisearch.New(host, opts...)

	health, err := cli.HealthWithContext(ctx)
	if err != nil {
		d.Logger(ctx).Error("meilisearch 组件探活失败", "error", err)
		return err
	}
	if health.Status != "available" {
		err = errors.Errorf("meilisearch 健康状态异常: %s", health.Status)
		d.Logger(ctx).Error("meilisearch 组件探活失败", "error", err)
		return err
	}

	d.cli = cli
	d.Logger(ctx).Debug("meilisearch 组件初始化完成", "host", conf.Host)
	return nil
}

func (d *db) Shutdown() error {
	// v0.36 ServiceManager 内部管理连接，未暴露关闭接口
	return nil
}

// hostFor 校验并规范化 Host，仅接受 `http://host:7700` / `https://host:7700` 形式。
func hostFor(host string) (string, error) {
	u, err := url.Parse(host)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.WithMessage(ErrInvalidHost, "host: "+host)
	}
	return u.Scheme + "://" + u.Host, nil
}
