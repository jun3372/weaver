# weaver components/nsq

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/components/nsq.svg)](https://pkg.go.dev/github.com/jun3372/weaver/components/nsq)

Weaver 的 NSQ 客户端组件，基于官方 [go-nsq](https://github.com/nsqio/go-nsq)，托管 Producer 初始化。声明 `weaver.Ref[nsq.T]` 即可注入。

## 安装

```bash
go get github.com/jun3372/weaver/components/nsq
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/components/nsq"
```

## 配置

```yaml
nsq:
  Enabled: true
  DSN: "nsq://localhost:4150"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `DSN` | `nsq://localhost:4150` | nsqd TCP 地址；多地址取第一个（go-nsq Producer 只指向单个 nsqd，断连重连由库内处理） |

环境变量覆盖：`WEAVER_NSQ_DSN`、`WEAVER_NSQ_ENABLED`。

## 使用示例

```go
type taskDispatcher struct {
    weaver.Implements[TaskDispatcher]
    weaver.Ref[nsq.T]
}

func (t *taskDispatcher) Dispatch(ctx context.Context, topic string, body []byte) error {
    prod, err := t.Ref().Client(ctx)
    if err != nil {
        return err
    }
    return prod.Publish(topic, body)
}
```

## 生命周期

- `Init`：未启用时打日志直接返回；创建 Producer 后 `Ping` 探活，失败自动 `Stop` 并返回错误
- `Shutdown`：`Stop` 并等待退出完成（`StopChan`）

> 当前仅托管 Producer；Consumer 需要 topic/channel 语义，按业务自行基于 `nsq.NewConsumer` 创建。

## 完整教程

更多用法见文档站的[消息队列教程](https://github.com/jun3372/weaver/blob/main/docs/guide/components/messaging.md#nsq)。
