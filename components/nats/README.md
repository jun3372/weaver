# weaver components/nats

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/nats.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/nats)

Weaver 的 NATS 客户端组件，基于官方 [nats.go](https://github.com/nats-io/nats.go)。声明 `weaver.Ref[nats.T]` 即可注入，连接建立、探活与优雅关闭由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/components/nats
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/nats"
```

## 配置

```yaml
nats:
  Enabled: true
  DSN: "nats://localhost:4222"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `nats://localhost:4222` | 连接串，多 server 逗号分隔：`nats://host1:4222,host2:4222`；支持 `nats://user:pass@host:4222` 认证与 URL 参数（如 `?timeout=5s`） |

环境变量覆盖：`WEAVER_NATS_DSN`、`WEAVER_NATS_ENABLED`。

## 使用示例

```go
type eventBus struct {
    weaver.Implements[EventBus]
    weaver.Ref[nats.T]
}

func (e *eventBus) Publish(ctx context.Context, subject string, data []byte) error {
    conn, err := e.Ref().Client(ctx)
    if err != nil {
        return err
    }
    return conn.Publish(subject, data)
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；`nats.Connect`（内部完成 PING/PONG 握手）后 `FlushTimeout` 二次探活，失败自动 `Close` 并返回错误
- `Shutdown`：`Drain` 优先于 `Close`，冲刷残留订阅后再退出

## 完整教程

更多用法见文档站的[消息队列教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/messaging.md#nats)。
