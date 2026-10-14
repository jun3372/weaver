# weaver components/kafka

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/kafka.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/kafka)

Weaver 的 Kafka 客户端组件，基于 [IBM/sarama](https://github.com/IBM/sarama)。声明 `weaver.Ref[kafka.T]` 即可注入，broker 连接、元数据探活与优雅关闭由框架生命周期托管。

## 安装

```bash
go get github.com/jun3372/weaver/components/kafka
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/kafka"
```

## 配置

```yaml
kafka:
  Enabled: true
  DSN: "kafka://localhost:9092"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `kafka://localhost:9092` | broker 列表，多 broker 用逗号分隔：`kafka://host1:9092,host2:9092` |

环境变量覆盖：`WEAVER_KAFKA_DSN`、`WEAVER_KAFKA_ENABLED`。

## 使用示例

```go
type orderProducer struct {
    weaver.Implements[OrderProducer]
    weaver.Ref[kafka.T]
}

func (o *orderProducer) Send(ctx context.Context, msg []byte) error {
    cli, err := o.Ref().Client(ctx)
    if err != nil {
        return err
    }
    // 基于 sarama.Client 派生 SyncProducer / Consumer 等
    _ = cli
    return nil
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；解析 DSN 得到 broker 列表后 `sarama.NewClient` 建连，并 `RefreshMetadata` 探活，失败时自动 `Close` 并返回错误
- `Shutdown`：`Close` 关闭客户端

> 当前仅托管 `sarama.Client` 初始化，SASL/TLS 等高级配置暂未暴露，按需在 `Config` 中扩展字段。

## 完整教程

更多用法见文档站的[消息队列教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/messaging.md#kafka)。
