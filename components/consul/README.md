# weaver components/consul

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/consul.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/consul)

Weaver 的 Consul 客户端组件，基于官方 [consul/api](https://github.com/hashicorp/consul)。声明 `weaver.Ref[consul.T]` 即可注入，client 配置、探活由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/components/consul
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/consul"
```

## 配置

```yaml
consul:
  Enabled: true
  DSN: "http://127.0.0.1:8500"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `http://127.0.0.1:8500` | Consul HTTP API 地址，scheme 仅支持 `http` / `https`；userinfo 转 Basic Auth，如 `http://admin:secret@127.0.0.1:8500` |

环境变量覆盖：`WEAVER_CONSUL_DSN`、`WEAVER_CONSUL_ENABLED`。

## 使用示例

```go
type serviceRegistry struct {
    weaver.Implements[ServiceRegistry]
    weaver.Ref[consul.T]
}

func (s *serviceRegistry) Services(ctx context.Context) (map[string][]string, error) {
    cli, err := s.Ref().Client(ctx)
    if err != nil {
        return nil, err
    }
    return cli.Catalog().Services(nil)
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；解析 DSN 填充 `api.DefaultConfig()` 后 `api.NewClient`，并 `Status().Leader()` 探活
- `Shutdown`：HTTP client 无需显式关闭

## 完整教程

更多用法见文档站的[注册与配置中心教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/registry.md#consul)。
