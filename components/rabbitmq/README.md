# weaver components/rabbitmq

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/rabbitmq.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/rabbitmq)

Weaver 的 RabbitMQ 客户端组件，基于 [amqp091-go](https://github.com/rabbitmq/amqp091-go)。声明 `weaver.Ref[rabbitmq.T]` 即可注入，连接建立、探活与优雅关闭由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/components/rabbitmq
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/rabbitmq"
```

## 配置

```yaml
rabbitmq:
  Enabled: true
  DSN: "amqp://guest:guest@localhost:5672/"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `amqp://guest:guest@localhost:5672/` | AMQP 连接串，scheme 仅支持 `amqp` / `amqps` |

环境变量覆盖：`WEAVER_RABBITMQ_DSN`、`WEAVER_RABBITMQ_ENABLED`。

## 使用示例

```go
type orderPublisher struct {
    weaver.Implements[OrderPublisher]
    weaver.Ref[rabbitmq.T]
}

func (o *orderPublisher) Publish(ctx context.Context, body []byte) error {
    conn, err := o.Get().Client(ctx)
    if err != nil {
        return err
    }
    ch, err := conn.Channel()
    if err != nil {
        return err
    }
    defer ch.Close()
    return ch.PublishWithContext(ctx, "orders", "", false, false,
        amqp.Publishing{ContentType: "application/json", Body: body})
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；`amqp.Dial` 建连后试开一条 Channel 探活，失败自动 `Close` 并返回错误
- `Shutdown`：`Close` 关闭连接

## 完整教程

更多用法见文档站的[消息队列教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/messaging.md#rabbitmq)。
