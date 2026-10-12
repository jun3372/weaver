package kafka

import (
	"context"
	"strings"

	"github.com/IBM/sarama"
	"github.com/jun3372/weaver"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (sarama.Client, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"kafka"`
	cli                       sarama.Client
}

func (d *db) Client(_ context.Context) (sarama.Client, error) {
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
		d.Logger(ctx).Info("kafka 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("kafka 组件初始化开始", "config", conf)
	brokers, err := brokersFor(conf.DSN)
	if err != nil {
		return err
	}

	cfg := sarama.NewConfig()
	cfg.Version = sarama.DefaultVersion
	cli, err := sarama.NewClient(brokers, cfg)
	if err != nil {
		d.Logger(ctx).Error("kafka 组件初始化失败", "error", err)
		return err
	}

	if err := cli.RefreshMetadata(); err != nil {
		_ = cli.Close()
		d.Logger(ctx).Error("kafka 组件元数据刷新失败", "error", err)
		return err
	}

	d.cli = cli
	d.Logger(ctx).Debug("kafka 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	if d.cli == nil {
		return nil
	}

	return d.cli.Close()
}

// brokersFor 兼容 `kafka://host:9092` 与 `kafka://host1:9092,host2:9092` 两种 DSN 形式。
func brokersFor(dsn string) ([]string, error) {
	scheme, rest, ok := strings.Cut(dsn, ":")
	if !ok || scheme != "kafka" {
		return nil, ErrInvalidDSN
	}
	rest = strings.TrimPrefix(rest, "//")

	var brokers []string
	for _, b := range strings.Split(rest, ",") {
		if b = strings.TrimSpace(b); b != "" {
			brokers = append(brokers, b)
		}
	}
	if len(brokers) == 0 {
		return nil, errors.WithMessage(ErrInvalidDSN, "无可用 broker")
	}
	return brokers, nil
}
