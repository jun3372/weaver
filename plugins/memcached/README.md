# weaver plugins/memcached

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/plugins/memcached.svg)](https://pkg.go.dev/github.com/jun3372/weaver/plugins/memcached)

Weaver 的 Memcached 客户端组件，基于 [bradfitz/gomemcache](https://github.com/bradfitz/gomemcache)。声明 `weaver.Ref[memcached.T]` 即可注入，节点配置与探活由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/plugins/memcached
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/plugins/memcached"
```

## 配置

```yaml
memcached:
  Enabled: true
  DSN: "memcache://localhost:11211"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `memcache://localhost:11211` | server 列表，多节点逗号分隔：`memcache://host1:11211,host2:11211` |

环境变量覆盖：`WEAVER_MEMCACHED_DSN`、`WEAVER_MEMCACHED_ENABLED`。

## 使用示例

```go
type hotCache struct {
    weaver.Implements[HotCache]
    weaver.Ref[memcached.T]
}

func (h *hotCache) Get(ctx context.Context, key string) ([]byte, error) {
    cli, err := h.Ref().Client(ctx)
    if err != nil {
        return nil, err
    }
    item, err := cli.Get(key)
    if errors.Is(err, memcache.ErrCacheMiss) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return item.Value, nil
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；解析 DSN 得到 server 列表后创建 client，并 `Ping` 全部节点探活
- `Shutdown`：gomemcache client 内部管理连接，无需显式关闭

## 完整教程

更多用法见文档站的[缓存教程](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/cache.md#memcached)。
