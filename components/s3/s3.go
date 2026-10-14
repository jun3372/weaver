package s3

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/jun3372/weaver"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (*minio.Client, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"s3"`
	cli                       *minio.Client
}

func (d *db) Client(_ context.Context) (*minio.Client, error) {
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
		d.Logger(ctx).Info("s3 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("s3 组件初始化开始", "config", conf)
	endpoint, ak, sk, bucket, secure, err := parseDSN(conf.DSN)
	if err != nil {
		return err
	}

	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(ak, sk, ""),
		Secure: secure,
	})
	if err != nil {
		d.Logger(ctx).Error("s3 组件初始化失败", "error", err)
		return err
	}

	// bucket 可选：配了才探活
	if bucket != "" {
		ok, err := cli.BucketExists(ctx, bucket)
		if err != nil {
			d.Logger(ctx).Error("s3 组件 bucket 探活失败", "bucket", bucket, "error", err)
			return err
		}
		if !ok {
			err = errors.WithMessage(ErrBucketNotFound, bucket)
			d.Logger(ctx).Error("s3 组件 bucket 不存在", "error", err)
			return err
		}
	}

	d.cli = cli
	d.Logger(ctx).Debug("s3 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	// minio client 无需显式关闭
	return nil
}

// parseDSN 兼容 `s3://accessKey:secretKey@endpoint` 与
// `s3://accessKey:secretKey@endpoint/bucket?secure=true` 两种形式，
// secure 未指定时默认 false（本地 MinIO 场景）。
func parseDSN(dsn string) (endpoint, ak, sk, bucket string, secure bool, err error) {
	u, perr := url.Parse(dsn)
	if perr != nil || u.Host == "" {
		return "", "", "", "", false, errors.WithMessage(ErrInvalidDSN, "dsn: "+dsn)
	}
	if u.Scheme != "s3" {
		return "", "", "", "", false, errors.WithMessage(ErrInvalidScheme, "scheme: "+u.Scheme)
	}
	if u.User == nil {
		return "", "", "", "", false, errors.WithMessage(ErrInvalidDSN, "缺少 accessKey/secretKey")
	}
	sk, _ = u.User.Password()

	bucket = strings.Trim(u.Path, "/")
	if q := u.Query().Get("secure"); q != "" {
		if secure, err = strconv.ParseBool(q); err != nil {
			return "", "", "", "", false, errors.WithMessage(ErrInvalidDSN, "secure: "+q)
		}
	}

	return u.Host, u.User.Username(), sk, bucket, secure, nil
}
