# weaver components/nacos

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/nacos.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/nacos)

Weaver 的 Nacos 客户端组件，基于官方 [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go)，同时托管服务发现（Naming）与配置中心（Config）两类客户端。声明 `weaver.Ref[nacos.T]` 即可注入。

## 安装

```bash
go get github.com/jun3372/weaver/components/nacos
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/nacos"
```

## 配置

```yaml
nacos:
  Enabled: true
  DSN: "nacos://nacos:nacos@localhost:8848?namespaceId=dev"
```

### DSN 格式

```
nacos://[user:pass@]host:8848[,host2:8848][?namespaceId=xxx]
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `nacos://localhost:8848` | server 列表逗号分隔；userinfo 转 username/password；`namespaceId` 指定命名空间；缺省端口 8848 |

环境变量覆盖：`WEAVER_NACOS_DSN`、`WEAVER_NACOS_ENABLED`。

## 使用示例

```go
type serviceGateway struct {
    weaver.Implements[ServiceGateway]
    weaver.Ref[nacos.T]
}

func (g *serviceGateway) Instances(ctx context.Context, service string) ([]model.Instance, error) {
    naming, err := g.Ref().NamingClient(ctx)
    if err != nil {
        return nil, err
    }
    res, err := naming.SelectInstances(vo.SelectAllInstancesParam{ServiceName: service})
    if err != nil {
        return nil, err
    }
    return res.Instances, nil
}

func (g *serviceGateway) LoadConfig(ctx context.Context, dataId, group string) (string, error) {
    config, err := g.Ref().ConfigClient(ctx)
    if err != nil {
        return "", err
    }
    content, err := config.GetConfig(vo.ConfigParam{DataId: dataId, Group: group})
    if err != nil {
        return "", err
    }
    return content, nil
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；依次创建 Naming / Config 客户端（5 秒超时、启动不加载本地缓存），随后以 `GetAllServicesInfo` 探活，任一失败自动 `Shutdown` 已创建的客户端并返回错误
- `Shutdown`：依次 `Shutdown` Naming / Config 客户端

## 完整教程

更多用法（服务注册与发现、配置监听）见文档站的[注册与配置中心教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/registry.md#nacos)。
