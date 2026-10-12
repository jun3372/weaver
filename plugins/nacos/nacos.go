package nacos

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/jun3372/weaver"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
)

type T interface {
	NamingClient(ctx context.Context) (naming_client.INamingClient, error)
	ConfigClient(ctx context.Context) (config_client.IConfigClient, error)
}

type db struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"nacos"`
	naming                    naming_client.INamingClient
	config                    config_client.IConfigClient
}

func (d *db) NamingClient(_ context.Context) (naming_client.INamingClient, error) {
	if !d.Config().Enabled {
		return nil, ErrNotEnabled
	}

	if d.naming == nil {
		return nil, ErrNotReady
	}

	return d.naming, nil
}

func (d *db) ConfigClient(_ context.Context) (config_client.IConfigClient, error) {
	if !d.Config().Enabled {
		return nil, ErrNotEnabled
	}

	if d.config == nil {
		return nil, ErrNotReady
	}

	return d.config, nil
}

func (d *db) Init(ctx context.Context) error {
	conf := d.Config()
	if !conf.Enabled {
		d.Logger(ctx).Info("nacos 组件未启用")
		return nil
	}

	d.Logger(ctx).Debug("nacos 组件初始化开始", "config", conf)
	servers, namespace, user, pass, err := parseDSN(conf.DSN)
	if err != nil {
		return err
	}

	cc := constant.NewClientConfig(
		constant.WithTimeoutMs(5_000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithNamespaceId(namespace),
		constant.WithUsername(user),
		constant.WithPassword(pass),
	)
	naming, err := clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  cc,
		ServerConfigs: servers,
	})
	if err != nil {
		d.Logger(ctx).Error("nacos 服务发现客户端初始化失败", "error", err)
		return err
	}

	config, err := clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  cc,
		ServerConfigs: servers,
	})
	if err != nil {
		naming.CloseClient()
		d.Logger(ctx).Error("nacos 配置中心客户端初始化失败", "error", err)
		return err
	}

	// 探活：拉取服务列表（空列表也是成功）
	if _, err := naming.GetAllServicesInfo(vo.GetAllServiceInfoParam{
		GroupName: constant.DEFAULT_GROUP,
		PageNo:    1,
		PageSize:  1,
	}); err != nil {
		naming.CloseClient()
		config.CloseClient()
		d.Logger(ctx).Error("nacos 组件探活失败", "error", err)
		return err
	}

	d.naming = naming
	d.config = config
	d.Logger(ctx).Debug("nacos 组件初始化完成", "conf", conf)
	return nil
}

func (d *db) Shutdown() error {
	if d.naming != nil {
		d.naming.CloseClient()
	}
	if d.config != nil {
		d.config.CloseClient()
	}
	return nil
}

// parseDSN 兼容 `nacos://[user:pass@]host:8848[,host2:8848][?namespaceId=xxx]`
// 形式，userinfo 转 username/password，缺省端口 8848。
func parseDSN(dsn string) (servers []constant.ServerConfig, namespace, user, pass string, err error) {
	u, perr := url.Parse(dsn)
	if perr != nil || u.Host == "" {
		return nil, "", "", "", errors.WithMessage(ErrInvalidDSN, "dsn: "+dsn)
	}
	if u.Scheme != "nacos" {
		return nil, "", "", "", errors.WithMessage(ErrInvalidScheme, "scheme: "+u.Scheme)
	}

	for _, host := range strings.Split(u.Host, ",") {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}

		ip, portStr, perr := net.SplitHostPort(host)
		if perr != nil {
			ip, portStr = host, "8848"
		}
		port, perr := strconv.ParseUint(portStr, 10, 64)
		if perr != nil {
			return nil, "", "", "", errors.WithMessage(ErrInvalidDSN, "端口: "+portStr)
		}
		servers = append(servers, *constant.NewServerConfig(ip, port, constant.WithContextPath("/nacos")))
	}
	if len(servers) == 0 {
		return nil, "", "", "", errors.WithMessage(ErrInvalidDSN, "无可用 server")
	}

	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	return servers, u.Query().Get("namespaceId"), user, pass, nil
}
