# weaver plugins/redis

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/plugins/redis.svg)](https://pkg.go.dev/github.com/jun3372/weaver/plugins/redis)

Weaver 的 Redis 客户端组件，基于 [go-redis/v9](https://github.com/redis/go-redis)。声明 `weaver.Ref[redis.T]` 即可注入，连接建立、Ping 探活与优雅关闭由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/plugins/redis
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/plugins/redis"
```

## 配置

```yaml
redis:
  Enabled: true
  DSN: "redis://localhost:6379/0"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `redis://localhost:6379/0` | Redis URL，支持 `redis://user:pass@host:port/db` 与 `rediss://`（TLS） |

环境变量覆盖：`WEAVER_REDIS_DSN`、`WEAVER_REDIS_ENABLED`。

## 使用示例

```go
type sessionStore struct {
    weaver.Implements[SessionStore]
    weaver.Ref[redis.T]
}

func (s *sessionStore) Set(ctx context.Context, key, val string) error {
    cli, err := s.Ref().Client(ctx)
    if err != nil {
        return err
    }
    return cli.Set(ctx, key, val, time.Hour).Err()
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；`ParseURL` 解析 DSN 后 `redis.NewClient`，并 `Ping` 探活，失败时自动 `Close` 并返回错误
- `Shutdown`：`Close` 关闭客户端

## 完整教程

更多用法（常用数据结构、Pipeline、Pub/Sub、客户端复用）见文档站的[缓存教程](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/cache.md#redis)。
