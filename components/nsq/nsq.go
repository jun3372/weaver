package nsq

import (
	"context"
	"strings"
	"time"

	"github.com/jun3372/weaver"
	"github.com/nsqio/go-nsq"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (*nsq.Producer, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"nsq"`
	prod                      *nsq.Producer
}

func (d *db) Client(_ context.Context) (*nsq.Producer, error) {
	if !d.Config().Enabled {
		return nil, ErrNotEnabled
	}

	if d.prod == nil {
		return nil, ErrNotReady
	}

	return d.prod, nil
}

func (d *db) Init(ctx context.Context) error {
	conf := d.Config()
	if !conf.Enabled {
		d.Logger(ctx).Info("nsq 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("nsq 组件初始化开始", "config", conf)
	addr, err := addrFor(conf.DSN)
	if err != nil {
		return err
	}

	cfg := nsq.NewConfig()
	cfg.DialTimeout = 5 * time.Second
	prod, err := nsq.NewProducer(addr, cfg)
	if err != nil {
		d.Logger(ctx).Error("nsq 组件初始化失败", "error", err)
		return err
	}

	if err := prod.Ping(); err != nil {
		prod.Stop()
		d.Logger(ctx).Error("nsq 组件探活失败", "error", err)
		return err
	}

	d.prod = prod
	d.Logger(ctx).Debug("nsq 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	if d.prod == nil {
		return nil
	}

	// Stop 是异步的，由库内保证在途消息处理完再退出
	d.prod.Stop()
	return nil
}

// addrFor 兼容 `nsq://host:4150` 形式，go-nsq 的 Producer 只指向单个 nsqd，
// 多地址取第一个（nsqd 故障转移由库内重连机制处理）。
func addrFor(dsn string) (string, error) {
	scheme, rest, ok := strings.Cut(dsn, ":")
	if !ok || scheme != "nsq" {
		return "", ErrInvalidDSN
	}
	rest = strings.TrimPrefix(rest, "//")

	var addr string
	for _, a := range strings.Split(rest, ",") {
		if a = strings.TrimSpace(a); a != "" {
			addr = a
			break
		}
	}
	if addr == "" {
		return "", errors.WithMessage(ErrInvalidDSN, "无可用地址")
	}
	return addr, nil
}
