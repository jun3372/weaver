# weaver components/etcd

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/etcd.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/etcd)

Weaver 的 etcd 客户端组件，基于官方 [client/v3](https://github.com/etcd-io/etcd)。声明 `weaver.Ref[etcd.T]` 即可注入，连接、Status 探活与优雅关闭由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/components/etcd
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/etcd"
```

## 配置

```yaml
etcd:
  Enabled: true
  DSN: "etcd://localhost:2379"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `etcd://localhost:2379` | endpoint 列表，多节点逗号分隔：`etcd://host1:2379,host2:2379` |

环境变量覆盖：`WEAVER_ETCD_DSN`、`WEAVER_ETCD_ENABLED`。

## 使用示例

```go
type configCenter struct {
    weaver.Implements[ConfigCenter]
    weaver.Ref[etcd.T]
}

func (c *configCenter) Get(ctx context.Context, key string) (string, error) {
    cli, err := c.Get().Client(ctx)
    if err != nil {
        return "", err
    }
    resp, err := cli.Get(ctx, key)
    if err != nil {
        return "", err
    }
    if len(resp.Kvs) == 0 {
        return "", nil
    }
    return string(resp.Kvs[0].Value), nil
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；解析 DSN 得到 endpoint 列表后 `clientv3.New`（`DialTimeout: 5s`），并对首个 endpoint 执行 `Status` 探活，失败时自动 `Close` 并返回错误
- `Shutdown`：`Close` 关闭客户端

## 完整教程

更多用法见文档站的[注册与配置中心教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/registry.md#etcd)。
