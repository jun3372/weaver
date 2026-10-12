# 消息队列:kafka / rabbitmq / nats / nsq / mqtt

Weaver 为五种常见消息中间件各提供一个客户端组件插件,遵循同一套约定:配置只暴露 `Enabled` 与连接 DSN(`mqtt` 额外有 `ClientID`),声明 `weaver.Ref[xxx.T]` 即可注入底层客户端,连接建立、探活与优雅关闭由框架生命周期托管。`Client(ctx)` 在未启用时返回 `ErrNotEnabled`,未就绪时返回 `ErrNotReady`,业务代码可用 `errors.Is` 统一降级。插件只托管客户端初始化,发布 / 订阅等业务语义按底层库原生 API 编写。

## kafka

Weaver 的 Kafka 客户端组件,基于 [IBM/sarama](https://github.com/IBM/sarama)。声明 `weaver.Ref[kafka.T]` 即可注入 `sarama.Client`,broker 连接与元数据探活由框架托管。

### 安装

安装:`go get github.com/jun3372/weaver/plugins/kafka`,在主包匿名导入 `import _ "github.com/jun3372/weaver/plugins/kafka"` 完成组件注册。

### 配置

```yaml
kafka:
  Enabled: true
  DSN: "kafka://localhost:9092"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `kafka://localhost:9092` | broker 列表,多 broker 逗号分隔:`kafka://host1:9092,host2:9092` |

环境变量覆盖:`WEAVER_KAFKA_DSN`、`WEAVER_KAFKA_ENABLED`。

### 基础用法

```go
type orderProducer struct {
    weaver.Implements[OrderProducer]
    weaver.Ref[kafka.T]
}

func (o *orderProducer) Send(ctx context.Context, topic string, body []byte) error {
    cli, err := o.Ref().Client(ctx)
    if err != nil {
        return err
    }
    return o.dispatch(cli, topic, body) // cli 即 sarama.Client
}
```

### 进阶场景

#### 同步生产者

```go
cli, _ := o.Ref().Client(ctx)
prod, err := sarama.NewSyncProducerFromClient(sarama.NewConfig(), cli)
if err != nil {
    return err
}
defer prod.Close()

_, _, err = prod.SendMessage(&sarama.ProducerMessage{
    Topic: "orders",
    Value: sarama.ByteEncoder(body),
})
```

#### 消费组

从同一个 `sarama.Client` 派生消费组,实现 `sarama.ConsumerGroupHandler` 处理消息(骨架):

```go
group, err := sarama.NewConsumerGroupFromClient("order-workers", cli)
if err != nil {
    return err
}
defer group.Close()
// Consume 会阻塞,建议放独立 goroutine;rebalance 后可循环重入
go func() {
    for {
        if err := group.Consume(ctx, []string{"orders"}, &orderHandler{}); err != nil {
            return
        }
    }
}()

type orderHandler struct{}
func (h *orderHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *orderHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *orderHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    for msg := range claim.Messages() {
        // 处理消息...
        sess.MarkMessage(msg, "") // 标记已消费,提交 offset
    }
    return nil
}
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;解析 DSN 得到 broker 列表后 `sarama.NewClient` 建连,并 `RefreshMetadata` 探活,失败时自动 `Close` 并返回错误
- `Shutdown`:`Close` 关闭客户端
- 常见错误:`ErrNotEnabled` / `ErrNotReady`(插件未启用/未就绪)、`ErrInvalidDSN`(DSN 非 `kafka://` scheme 或无可用 broker)

## rabbitmq

Weaver 的 RabbitMQ 客户端组件,基于 [amqp091-go](https://github.com/rabbitmq/amqp091-go)。声明 `weaver.Ref[rabbitmq.T]` 即可注入 `*amqp.Connection`,连接建立与探活由框架托管。

### 安装

安装:`go get github.com/jun3372/weaver/plugins/rabbitmq`,在主包匿名导入 `import _ "github.com/jun3372/weaver/plugins/rabbitmq"` 完成组件注册。

### 配置

```yaml
rabbitmq:
  Enabled: true
  DSN: "amqp://guest:guest@localhost:5672/"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `amqp://guest:guest@localhost:5672/` | AMQP 连接串,scheme 仅支持 `amqp` / `amqps` |

环境变量覆盖:`WEAVER_RABBITMQ_DSN`、`WEAVER_RABBITMQ_ENABLED`。

### 基础用法

```go
type orderPublisher struct {
    weaver.Implements[OrderPublisher]
    weaver.Ref[rabbitmq.T]
}

func (o *orderPublisher) Publish(ctx context.Context, body []byte) error {
    conn, err := o.Ref().Client(ctx)
    if err != nil {
        return err
    }
    ch, err := conn.Channel() // Channel 轻量,用完即关
    if err != nil {
        return err
    }
    defer ch.Close()
    return ch.PublishWithContext(ctx, "orders", "", false, false,
        amqp.Publishing{ContentType: "application/json", Body: body})
}
```

### 进阶场景

#### 声明 Exchange 与 Queue 并绑定

```go
ch, _ := conn.Channel()
defer ch.Close()
_ = ch.ExchangeDeclare("orders", "topic", true, false, false, false, nil)
_ = ch.QueueDeclare("orders.payment", true, false, false, false, nil)
_ = ch.QueueBind("orders.payment", "order.paid.*", "orders", false, nil)
```

#### 消费与 ack

```go
msgs, err := ch.Consume("orders.payment", "payment-worker",
    false, // autoAck:设 false 手动确认,处理失败可不 ack 重新入队
    false, false, false, nil)
if err != nil {
    return err
}
for d := range msgs {
    // 处理消息...
    _ = d.Ack(false) // 处理成功后确认;失败可 d.Nack(false, true) 重新入队
}
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;`amqp.Dial` 建连后试开一条 Channel 探活,失败自动 `Close` 并返回错误
- `Shutdown`:`Close` 关闭连接
- 常见错误:`ErrNotEnabled` / `ErrNotReady`(插件未启用/未就绪);Connection 复用框架托管的同一实例,Channel 每次操作自行创建

## nats

Weaver 的 NATS 客户端组件,基于官方 [nats.go](https://github.com/nats-io/nats.go)。声明 `weaver.Ref[nats.T]` 即可注入 `*nats.Conn`,连接建立与探活由框架托管。

### 安装

安装:`go get github.com/jun3372/weaver/plugins/nats`,在主包匿名导入 `import _ "github.com/jun3372/weaver/plugins/nats"` 完成组件注册。

### 配置

```yaml
nats:
  Enabled: true
  DSN: "nats://localhost:4222"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `nats://localhost:4222` | 连接串,多 server 逗号分隔:`nats://host1:4222,host2:4222`;支持 `nats://user:pass@host:4222` 认证 |

环境变量覆盖:`WEAVER_NATS_DSN`、`WEAVER_NATS_ENABLED`。

### 基础用法

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

### 进阶场景

#### 订阅与队列订阅

```go
conn, _ := e.Ref().Client(ctx)
// 普通订阅:每个订阅者都收到全量消息
sub, err := conn.Subscribe("events.order.*", func(m *nats.Msg) {
    fmt.Println(m.Subject, string(m.Data))
})
defer sub.Unsubscribe()

// 队列订阅:同 queue group 内负载均衡,每条消息只投递给组内一个成员
qsub, _ := conn.QueueSubscribe("events.order.*", "workers", func(m *nats.Msg) {})
defer qsub.Unsubscribe()
```

#### Request/Reply 同步调用

```go
// 请求方:等待应答,超时返回错误
reply, err := conn.Request("svc.user.get", []byte(`{"id":1}`), 2*time.Second)
// 应答方:收到请求后回复
conn.Subscribe("svc.user.get", func(m *nats.Msg) {
    _ = m.Respond([]byte(`{"name":"alice"}`))
})
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;`nats.Connect`(内部完成 PING/PONG 握手)后 `FlushTimeout` 二次探活,失败自动 `Close` 并返回错误
- `Shutdown`:`Drain` 优先于 `Close`,冲刷残留订阅后再退出
- 常见错误:`ErrNotEnabled` / `ErrNotReady`(插件未启用/未就绪)

## nsq

Weaver 的 NSQ 客户端组件,基于官方 [go-nsq](https://github.com/nsqio/go-nsq),托管 Producer 初始化。声明 `weaver.Ref[nsq.T]` 即可注入 `*nsq.Producer`。

### 安装

安装:`go get github.com/jun3372/weaver/plugins/nsq`,在主包匿名导入 `import _ "github.com/jun3372/weaver/plugins/nsq"` 完成组件注册。

### 配置

```yaml
nsq:
  Enabled: true
  DSN: "nsq://localhost:4150"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `nsq://localhost:4150` | nsqd TCP 地址;多地址取第一个(go-nsq Producer 只指向单个 nsqd,断连重连由库内处理) |

环境变量覆盖:`WEAVER_NSQ_DSN`、`WEAVER_NSQ_ENABLED`。

### 基础用法

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

### 进阶场景

插件只托管 Producer;Consumer 涉及 topic/channel 语义,按业务自行基于 `nsq.NewConsumer` 创建,注意 `AddHandler` 必须在连接之前调用。生产侧另有 `prod.DeferredPublish(topic, delay, body)` 延迟投递(常用于重试退避)与 `prod.MultiPublish(topic, [][]byte{...})` 批量发布:

```go
cfg := nsq.NewConfig()
consumer, err := nsq.NewConsumer("tasks", "worker", cfg)
if err != nil {
    return err
}
consumer.AddHandler(nsq.HandlerFunc(func(m *nsq.Message) error {
    // 处理消息;返回 error 触发重试(受 MaxAttempts 限制)
    return nil
}))
// 消费端需自行 ConnectToNSQD 连接 nsqd(地址即 DSN 去掉 nsq:// 前缀)
if err := consumer.ConnectToNSQD("localhost:4150"); err != nil {
    return err
}
defer consumer.Stop()
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;解析 DSN 后 `nsq.NewProducer` 建连,并 `Ping` 探活,失败自动 `Stop` 并返回错误
- `Shutdown`:`Stop` 由库内保证在途消息处理完再退出
- 常见错误:`ErrNotEnabled` / `ErrNotReady`(插件未启用/未就绪)、`ErrInvalidDSN`(DSN 非 `nsq://` scheme 或无可用地址)

## mqtt

Weaver 的 MQTT 客户端组件,基于 [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang),适用于 IoT 消息场景。声明 `weaver.Ref[mqtt.T]` 即可注入 `mqtt.Client`,连接建立与优雅断开由框架托管。

### 安装

安装:`go get github.com/jun3372/weaver/plugins/mqtt`,在主包匿名导入 `import _ "github.com/jun3372/weaver/plugins/mqtt"` 完成组件注册。

### 配置

```yaml
mqtt:
  Enabled: true
  DSN: "mqtt://localhost:1883"
  ClientID: "weaver-app"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `mqtt://localhost:1883` | broker 地址,scheme 支持 `mqtt` / `tcp` / `ssl` / `tls` / `ws` / `wss`;userinfo 转 broker 认证 |
| `ClientID` | `weaver` | MQTT ClientID,同一 broker 集群内需唯一 |

环境变量覆盖:`WEAVER_MQTT_DSN`、`WEAVER_MQTT_CLIENTID`、`WEAVER_MQTT_ENABLED`。

### 基础用法

```go
type deviceGateway struct {
    weaver.Implements[DeviceGateway]
    weaver.Ref[mqtt.T]
}

func (g *deviceGateway) Publish(ctx context.Context, topic string, payload []byte) error {
    cli, err := g.Ref().Client(ctx)
    if err != nil {
        return err
    }
    token := cli.Publish(topic, 1, false, payload)
    token.Wait() // 阻塞等待 broker 确认
    return token.Error()
}
```

### 进阶场景

#### 订阅与回调

```go
cli, _ := g.Ref().Client(ctx)
token := cli.Subscribe("device/+/status", 1, func(_ mqtt.Client, msg mqtt.Message) {
    fmt.Println(msg.Topic(), string(msg.Payload()))
})
token.Wait()
if err := token.Error(); err != nil {
    return err
}
defer cli.Unsubscribe("device/+/status")
```

#### QoS 等级说明

`Publish` / `Subscribe` 的 qos 参数含义,另 `Publish` 的 retained 为 `true` 时 broker 保留最后一条消息,新订阅者上线即收到,常用于设备状态类主题:

| QoS | 语义 | 适用场景 |
|---|---|---|
| `0` | 至多一次,不确认 | 高频传感器读数,允许丢失 |
| `1` | 至少一次,PUBACK 确认 | 大多数业务消息,可能重复需幂等 |
| `2` | 恰好一次,四次握手 | 计费、指令下发等不容丢失也不容重复的场景,开销最大 |

### 生命周期与错误

- `Init`:未启用时打日志直接返回;校验 DSN scheme 后建连(5 秒超时),失败自动 `Disconnect` 并返回错误
- `Shutdown`:`Disconnect(250ms)`,给在途消息留冲刷时间
- 常见错误:`ErrNotEnabled` / `ErrNotReady`(插件未启用/未就绪)、`ErrInvalidDSN` / `ErrInvalidScheme`(DSN 解析失败或 scheme 不受支持)

## 相关阅读

- [插件总览](/guide/plugins)
- [配置管理](/guide/config)
