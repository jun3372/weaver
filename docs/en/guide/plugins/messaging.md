# Messaging: kafka / rabbitmq / nats / nsq / mqtt

Weaver provides a client component plugin for each of five common message middlewares, all following the same conventions: configuration only exposes `Enabled` and the connection DSN (`mqtt` additionally has `ClientID`); declare `weaver.Ref[xxx.T]` to inject the underlying client, with connection establishment, health checks, and graceful shutdown managed by the framework lifecycle. `Client(ctx)` returns `ErrNotEnabled` when the plugin is not enabled and `ErrNotReady` when it is not ready, allowing business code to degrade uniformly via `errors.Is`. The plugins only manage client initialization; business semantics such as publishing / subscribing are written using the native APIs of the underlying libraries.

## kafka

Weaver's Kafka client component, built on [IBM/sarama](https://github.com/IBM/sarama). Declare `weaver.Ref[kafka.T]` to inject a `sarama.Client`; broker connections and metadata health checks are managed by the framework.

### Installation

Install: `go get github.com/jun3372/weaver/plugins/kafka`, then register the component by anonymously importing it in the main package with `import _ "github.com/jun3372/weaver/plugins/kafka"`.

### Configuration

```yaml
kafka:
  Enabled: true
  DSN: "kafka://localhost:9092"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `kafka://localhost:9092` | Broker list, multiple brokers separated by commas: `kafka://host1:9092,host2:9092` |

Environment variable overrides: `WEAVER_KAFKA_DSN`, `WEAVER_KAFKA_ENABLED`.

### Basic Usage

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
    return o.dispatch(cli, topic, body) // cli is a sarama.Client
}
```

### Advanced Scenarios

#### Synchronous Producer

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

#### Consumer Group

Derive a consumer group from the same `sarama.Client` and implement `sarama.ConsumerGroupHandler` to process messages (skeleton):

```go
group, err := sarama.NewConsumerGroupFromClient("order-workers", cli)
if err != nil {
    return err
}
defer group.Close()
// Consume blocks; run it in a separate goroutine and loop back in after a rebalance
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
        // process message...
        sess.MarkMessage(msg, "") // mark as consumed and commit the offset
    }
    return nil
}
```

### Lifecycle & Errors

- `Init`: logs and returns directly when not enabled; parses the DSN into a broker list, calls `sarama.NewClient` to connect, and performs a `RefreshMetadata` health check; on failure, it automatically calls `Close` and returns an error
- `Shutdown`: `Close` closes the client
- Common errors: `ErrNotEnabled` / `ErrNotReady` (plugin not enabled/not ready), `ErrInvalidDSN` (DSN is not a `kafka://` scheme or no broker is reachable)

## rabbitmq

Weaver's RabbitMQ client component, built on [amqp091-go](https://github.com/rabbitmq/amqp091-go). Declare `weaver.Ref[rabbitmq.T]` to inject an `*amqp.Connection`; connection establishment and health checks are managed by the framework.

### Installation

Install: `go get github.com/jun3372/weaver/plugins/rabbitmq`, then register the component by anonymously importing it in the main package with `import _ "github.com/jun3372/weaver/plugins/rabbitmq"`.

### Configuration

```yaml
rabbitmq:
  Enabled: true
  DSN: "amqp://guest:guest@localhost:5672/"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `amqp://guest:guest@localhost:5672/` | AMQP connection string; the scheme only supports `amqp` / `amqps` |

Environment variable overrides: `WEAVER_RABBITMQ_DSN`, `WEAVER_RABBITMQ_ENABLED`.

### Basic Usage

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
    ch, err := conn.Channel() // Channel is lightweight; close it when done
    if err != nil {
        return err
    }
    defer ch.Close()
    return ch.PublishWithContext(ctx, "orders", "", false, false,
        amqp.Publishing{ContentType: "application/json", Body: body})
}
```

### Advanced Scenarios

#### Declaring Exchanges and Queues with Binding

```go
ch, _ := conn.Channel()
defer ch.Close()
_ = ch.ExchangeDeclare("orders", "topic", true, false, false, false, nil)
_ = ch.QueueDeclare("orders.payment", true, false, false, false, nil)
_ = ch.QueueBind("orders.payment", "order.paid.*", "orders", false, nil)
```

#### Consuming and Acknowledging

```go
msgs, err := ch.Consume("orders.payment", "payment-worker",
    false, // autoAck: set to false for manual acknowledgement; unacked messages are requeued on failure
    false, false, false, nil)
if err != nil {
    return err
}
for d := range msgs {
    // process message...
    _ = d.Ack(false) // acknowledge after successful processing; on failure use d.Nack(false, true) to requeue
}
```

### Lifecycle & Errors

- `Init`: logs and returns directly when not enabled; after `amqp.Dial`, trial-opens a Channel as a health check; on failure, it automatically calls `Close` and returns an error
- `Shutdown`: `Close` closes the connection
- Common errors: `ErrNotEnabled` / `ErrNotReady` (plugin not enabled/not ready); the Connection is a single framework-managed shared instance, while Channels are created per operation

## nats

Weaver's NATS client component, built on the official [nats.go](https://github.com/nats-io/nats.go). Declare `weaver.Ref[nats.T]` to inject a `*nats.Conn`; connection establishment and health checks are managed by the framework.

### Installation

Install: `go get github.com/jun3372/weaver/plugins/nats`, then register the component by anonymously importing it in the main package with `import _ "github.com/jun3372/weaver/plugins/nats"`.

### Configuration

```yaml
nats:
  Enabled: true
  DSN: "nats://localhost:4222"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `nats://localhost:4222` | Connection string, multiple servers separated by commas: `nats://host1:4222,host2:4222`; supports authentication via `nats://user:pass@host:4222` |

Environment variable overrides: `WEAVER_NATS_DSN`, `WEAVER_NATS_ENABLED`.

### Basic Usage

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

### Advanced Scenarios

#### Subscriptions and Queue Subscriptions

```go
conn, _ := e.Ref().Client(ctx)
// Plain subscription: every subscriber receives all messages
sub, err := conn.Subscribe("events.order.*", func(m *nats.Msg) {
    fmt.Println(m.Subject, string(m.Data))
})
defer sub.Unsubscribe()

// Queue subscription: load-balanced within the same queue group; each message is delivered to only one member
qsub, _ := conn.QueueSubscribe("events.order.*", "workers", func(m *nats.Msg) {})
defer qsub.Unsubscribe()
```

#### Request/Reply Synchronous Calls

```go
// Requester: waits for a reply, returns an error on timeout
reply, err := conn.Request("svc.user.get", []byte(`{"id":1}`), 2*time.Second)
// Responder: replies upon receiving a request
conn.Subscribe("svc.user.get", func(m *nats.Msg) {
    _ = m.Respond([]byte(`{"name":"alice"}`))
})
```

### Lifecycle & Errors

- `Init`: logs and returns directly when not enabled; after `nats.Connect` (which performs the PING/PONG handshake internally), performs a second health check via `FlushTimeout`; on failure, it automatically calls `Close` and returns an error
- `Shutdown`: `Drain` takes precedence over `Close`, flushing remaining subscriptions before exiting
- Common errors: `ErrNotEnabled` / `ErrNotReady` (plugin not enabled/not ready)

## nsq

Weaver's NSQ client component, built on the official [go-nsq](https://github.com/nsqio/go-nsq), managing Producer initialization. Declare `weaver.Ref[nsq.T]` to inject an `*nsq.Producer`.

### Installation

Install: `go get github.com/jun3372/weaver/plugins/nsq`, then register the component by anonymously importing it in the main package with `import _ "github.com/jun3372/weaver/plugins/nsq"`.

### Configuration

```yaml
nsq:
  Enabled: true
  DSN: "nsq://localhost:4150"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `nsq://localhost:4150` | nsqd TCP address; only the first address is used for multiple addresses (the go-nsq Producer points to a single nsqd; reconnection is handled inside the library) |

Environment variable overrides: `WEAVER_NSQ_DSN`, `WEAVER_NSQ_ENABLED`.

### Basic Usage

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

### Advanced Scenarios

The plugin only manages the Producer; a Consumer involves topic/channel semantics, so create it yourself based on `nsq.NewConsumer` according to your business needs. Note that `AddHandler` must be called before connecting. On the production side, `prod.DeferredPublish(topic, delay, body)` provides delayed delivery (commonly used for retry backoff) and `prod.MultiPublish(topic, [][]byte{...})` provides batch publishing:

```go
cfg := nsq.NewConfig()
consumer, err := nsq.NewConsumer("tasks", "worker", cfg)
if err != nil {
    return err
}
consumer.AddHandler(nsq.HandlerFunc(func(m *nsq.Message) error {
    // process message; return an error to trigger a retry (bounded by MaxAttempts)
    return nil
}))
// The consumer side needs to connect to nsqd itself via ConnectToNSQD (the address is the DSN minus the nsq:// prefix)
if err := consumer.ConnectToNSQD("localhost:4150"); err != nil {
    return err
}
defer consumer.Stop()
```

### Lifecycle & Errors

- `Init`: logs and returns directly when not enabled; parses the DSN and calls `nsq.NewProducer` to connect, then performs a `Ping` health check; on failure, it automatically calls `Stop` and returns an error
- `Shutdown`: `Stop` guarantees in-flight messages are processed before exiting
- Common errors: `ErrNotEnabled` / `ErrNotReady` (plugin not enabled/not ready), `ErrInvalidDSN` (DSN is not a `nsq://` scheme or no address is reachable)

## mqtt

Weaver's MQTT client component, built on [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang), suited for IoT messaging scenarios. Declare `weaver.Ref[mqtt.T]` to inject an `mqtt.Client`; connection establishment and graceful disconnection are managed by the framework.

### Installation

Install: `go get github.com/jun3372/weaver/plugins/mqtt`, then register the component by anonymously importing it in the main package with `import _ "github.com/jun3372/weaver/plugins/mqtt"`.

### Configuration

```yaml
mqtt:
  Enabled: true
  DSN: "mqtt://localhost:1883"
  ClientID: "weaver-app"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `mqtt://localhost:1883` | Broker address; the scheme supports `mqtt` / `tcp` / `ssl` / `tls` / `ws` / `wss`; userinfo maps to broker authentication |
| `ClientID` | `weaver` | MQTT ClientID, must be unique within the same broker cluster |

Environment variable overrides: `WEAVER_MQTT_DSN`, `WEAVER_MQTT_CLIENTID`, `WEAVER_MQTT_ENABLED`.

### Basic Usage

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
    token.Wait() // block until the broker acknowledges
    return token.Error()
}
```

### Advanced Scenarios

#### Subscriptions and Callbacks

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

#### QoS Levels

The meaning of the qos parameter for `Publish` / `Subscribe`. Additionally, when `Publish`'s retained flag is `true`, the broker keeps the last message and delivers it immediately to new subscribers — commonly used for device status topics:

| QoS | Semantics | Use Cases |
|---|---|---|
| `0` | At most once, no acknowledgement | High-frequency sensor readings where loss is acceptable |
| `1` | At least once, acknowledged via PUBACK | Most business messages; duplicates are possible, so handlers must be idempotent |
| `2` | Exactly once, four-way handshake | Scenarios like billing or command dispatch where neither loss nor duplication is tolerable; highest overhead |

### Lifecycle & Errors

- `Init`: logs and returns directly when not enabled; validates the DSN scheme and connects (5-second timeout); on failure, it automatically calls `Disconnect` and returns an error
- `Shutdown`: `Disconnect(250ms)`, leaving time for in-flight messages to flush
- Common errors: `ErrNotEnabled` / `ErrNotReady` (plugin not enabled/not ready), `ErrInvalidDSN` / `ErrInvalidScheme` (DSN parsing failed or unsupported scheme)

## Further Reading

- [Plugins Overview](/en/guide/plugins)
- [Configuration](/en/guide/config)
