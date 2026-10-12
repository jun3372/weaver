package mqtt

import (
	"context"
	"net/url"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/jun3372/weaver"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (mqtt.Client, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"mqtt"`
	cli                       mqtt.Client
}

func (d *db) Client(_ context.Context) (mqtt.Client, error) {
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
		d.Logger(ctx).Info("mqtt 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("mqtt 组件初始化开始", "config", conf)
	if err := validateDSN(conf.DSN); err != nil {
		return err
	}

	u, _ := url.Parse(conf.DSN)
	opts := mqtt.NewClientOptions().
		AddBroker(conf.DSN).
		SetClientID(conf.ClientID).
		SetConnectTimeout(5 * time.Second)
	if u.User != nil {
		opts.SetUsername(u.User.Username())
		pass, _ := u.User.Password()
		opts.SetPassword(pass)
	}

	cli := mqtt.NewClient(opts)
	tok := cli.Connect()
	if !tok.WaitTimeout(5 * time.Second) {
		cli.Disconnect(0)
		err := errors.WithMessage(ErrNotReady, "连接超时")
		d.Logger(ctx).Error("mqtt 组件连接超时", "error", err)
		return err
	}
	if err := tok.Error(); err != nil {
		d.Logger(ctx).Error("mqtt 组件初始化失败", "error", err)
		return err
	}

	d.cli = cli
	d.Logger(ctx).Debug("mqtt 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	if d.cli == nil {
		return nil
	}

	// quiesce 留 250ms 给在途消息冲刷
	d.cli.Disconnect(250)
	return nil
}

// validateDSN 兼容 `mqtt://`、`tcp://`、`ssl://`、`ws://` 等 paho 支持的 scheme，
// userinfo 转 broker 认证。
func validateDSN(dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return errors.WithMessage(ErrInvalidDSN, "dsn: "+dsn)
	}

	switch u.Scheme {
	case "mqtt", "tcp", "ssl", "tls", "ws", "wss":
		return nil
	default:
		return errors.WithMessage(ErrInvalidScheme, "scheme: "+u.Scheme)
	}
}
