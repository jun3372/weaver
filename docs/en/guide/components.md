# Components: Middleware Client Integrations

The `components/` directory provides 17 officially maintained middleware client components. Each component is an independent Go module that wraps the client of a common middleware into a weaver component: declare a `weaver.Ref[xxx.T]` and it gets injected — connection establishment (`Init`) and graceful shutdown (`Shutdown`) are managed by the framework lifecycle. All client libraries are pure Go implementations, so `CGO_ENABLED=0` cross-compilation works out of the box.

## Component List

| Category | Component | Client Library | Import Path |
|---|---|---|---|
| Database | [db](/en/guide/components/database#db) | [GORM](https://github.com/go-gorm/gorm) | `github.com/jun3372/weaver/components/db` |
| Database | [mongodb](/en/guide/components/database#mongodb) | [mongo-driver/v2](https://github.com/mongodb/mongo-go-driver) | `github.com/jun3372/weaver/components/mongodb` |
| Cache | [redis](/en/guide/components/cache#redis) | [go-redis/v9](https://github.com/redis/go-redis) | `github.com/jun3372/weaver/components/redis` |
| Cache | [memcached](/en/guide/components/cache#memcached) | [gomemcache](https://github.com/bradfitz/gomemcache) | `github.com/jun3372/weaver/components/memcached` |
| Messaging | [kafka](/en/guide/components/messaging#kafka) | [sarama](https://github.com/IBM/sarama) | `github.com/jun3372/weaver/components/kafka` |
| Messaging | [rabbitmq](/en/guide/components/messaging#rabbitmq) | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | `github.com/jun3372/weaver/components/rabbitmq` |
| Messaging | [nats](/en/guide/components/messaging#nats) | [nats.go](https://github.com/nats-io/nats.go) | `github.com/jun3372/weaver/components/nats` |
| Messaging | [nsq](/en/guide/components/messaging#nsq) | [go-nsq](https://github.com/nsqio/go-nsq) | `github.com/jun3372/weaver/components/nsq` |
| Messaging | [mqtt](/en/guide/components/messaging#mqtt) | [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) | `github.com/jun3372/weaver/components/mqtt` |
| Registry / Config Center | [consul](/en/guide/components/registry#consul) | [consul/api](https://github.com/hashicorp/consul) | `github.com/jun3372/weaver/components/consul` |
| Registry / Config Center | [etcd](/en/guide/components/registry#etcd) | [etcd clientv3](https://github.com/etcd-io/etcd) | `github.com/jun3372/weaver/components/etcd` |
| Registry / Config Center | [nacos](/en/guide/components/registry#nacos) | [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go) | `github.com/jun3372/weaver/components/nacos` |
| Registry / Config Center | [zookeeper](/en/guide/components/registry#zookeeper) | [go-zookeeper](https://github.com/go-zookeeper/zk) | `github.com/jun3372/weaver/components/zookeeper` |
| Object Storage | [s3](/en/guide/components/storage#s3) | [minio-go/v7](https://github.com/minio/minio-go) | `github.com/jun3372/weaver/components/s3` |
| Search | [elasticsearch](/en/guide/components/storage#elasticsearch) | [go-elasticsearch/v9](https://github.com/elastic/go-elasticsearch) | `github.com/jun3372/weaver/components/elasticsearch` |
| Search | [meilisearch](/en/guide/components/storage#meilisearch) | [meilisearch-go](https://github.com/meilisearch/meilisearch-go) | `github.com/jun3372/weaver/components/meilisearch` |
| Task Scheduling | [gocron](/en/guide/components/scheduler#gocron) | [go-co-op/gocron/v2](https://github.com/go-co-op/gocron) | `github.com/jun3372/weaver/components/gocron` |

## Uniform Usage Pattern

All components follow the same three-step pattern, using redis as an example:

```go
import (
    "github.com/jun3372/weaver"
    _ "github.com/jun3372/weaver/components/redis" // blank import completes component registration
)

type sessionStore struct {
    weaver.Implements[SessionStore]
    weaver.Ref[redis.T] // declare the dependency; the framework injects it automatically
}

func (s *sessionStore) Set(ctx context.Context, key, val string) error {
    cli, err := s.Get().Client(ctx) // obtain the underlying client
    if err != nil {
        return err
    }
    return cli.Set(ctx, key, val, time.Hour).Err()
}
```

```yaml
redis:
  Enabled: true
  DSN: "redis://localhost:6379/0"
```

## Common Conventions

### Configuration

Each component's configuration has only two fields, read from the config section of the same name (e.g. `redis:`, `kafka:`):

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the component is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | Varies per component | Connection string in `scheme://host:port` form; see each component's tutorial |

Environment variables can override the config file (e.g. `WEAVER_REDIS_DSN`, `WEAVER_REDIS_ENABLED`); see [Configuration](/en/guide/config) for the rules.

### Errors

Each component exposes two sentinel errors via `errors.go`, which can be checked directly with `errors.Is`:

| Error | Meaning |
|---|---|
| `ErrNotEnabled` | `Enabled: false` in the configuration; the component was not initialized |
| `ErrNotReady` | Enabled but the connection is not yet ready (e.g. after a failed `Init`) |

### Lifecycle

- `Init`: if not enabled, logs and returns immediately; parses the DSN → establishes the connection → probes with `Ping`. Any failure returns an error and aborts startup
- `Shutdown`: closes the underlying client connection; safe for components that were never initialized (not enabled)

### Dependency Isolation

Each component has its own `go.mod`; unless imported, no client library dependency is pulled in. Components with `Enabled: false` (the default) never establish a connection even if imported.

## Category Tutorials

- [Database: db / mongodb](/en/guide/components/database)
- [Cache: redis / memcached](/en/guide/components/cache)
- [Messaging: kafka / rabbitmq / nats / nsq / mqtt](/en/guide/components/messaging)
- [Registry & Config Center: consul / etcd / nacos / zookeeper](/en/guide/components/registry)
- [Storage & Search: s3 / elasticsearch / meilisearch](/en/guide/components/storage)
- [Task Scheduling: gocron](/en/guide/components/scheduler)

## Further Reading

- [Configuration](/en/guide/config)
- [Lifecycle Management](/en/guide/lifecycle)
