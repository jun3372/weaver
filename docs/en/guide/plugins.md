# Plugins: Middleware Client Integrations

The `plugins/` directory provides 15 officially maintained middleware client plugins. Each plugin is an independent Go module that wraps the client of a common middleware into a weaver component: declare a `weaver.Ref[xxx.T]` and it gets injected — connection establishment (`Init`) and graceful shutdown (`Shutdown`) are managed by the framework lifecycle. All client libraries are pure Go implementations, so `CGO_ENABLED=0` cross-compilation works out of the box.

## Plugin List

| Category | Plugin | Client Library | Import Path |
|---|---|---|---|
| Database | [db](/en/guide/plugins/database#db) | [GORM](https://github.com/go-gorm/gorm) | `github.com/jun3372/weaver/plugins/db` |
| Database | [mongodb](/en/guide/plugins/database#mongodb) | [mongo-driver/v2](https://github.com/mongodb/mongo-go-driver) | `github.com/jun3372/weaver/plugins/mongodb` |
| Cache | [redis](/en/guide/plugins/cache#redis) | [go-redis/v9](https://github.com/redis/go-redis) | `github.com/jun3372/weaver/plugins/redis` |
| Cache | [memcached](/en/guide/plugins/cache#memcached) | [gomemcache](https://github.com/bradfitz/gomemcache) | `github.com/jun3372/weaver/plugins/memcached` |
| Messaging | [kafka](/en/guide/plugins/messaging#kafka) | [sarama](https://github.com/IBM/sarama) | `github.com/jun3372/weaver/plugins/kafka` |
| Messaging | [rabbitmq](/en/guide/plugins/messaging#rabbitmq) | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | `github.com/jun3372/weaver/plugins/rabbitmq` |
| Messaging | [nats](/en/guide/plugins/messaging#nats) | [nats.go](https://github.com/nats-io/nats.go) | `github.com/jun3372/weaver/plugins/nats` |
| Messaging | [nsq](/en/guide/plugins/messaging#nsq) | [go-nsq](https://github.com/nsqio/go-nsq) | `github.com/jun3372/weaver/plugins/nsq` |
| Messaging | [mqtt](/en/guide/plugins/messaging#mqtt) | [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) | `github.com/jun3372/weaver/plugins/mqtt` |
| Registry / Config Center | [consul](/en/guide/plugins/registry#consul) | [consul/api](https://github.com/hashicorp/consul) | `github.com/jun3372/weaver/plugins/consul` |
| Registry / Config Center | [etcd](/en/guide/plugins/registry#etcd) | [etcd clientv3](https://github.com/etcd-io/etcd) | `github.com/jun3372/weaver/plugins/etcd` |
| Registry / Config Center | [nacos](/en/guide/plugins/registry#nacos) | [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go) | `github.com/jun3372/weaver/plugins/nacos` |
| Registry / Config Center | [zookeeper](/en/guide/plugins/registry#zookeeper) | [go-zookeeper](https://github.com/go-zookeeper/zk) | `github.com/jun3372/weaver/plugins/zookeeper` |
| Object Storage | [s3](/en/guide/plugins/storage#s3) | [minio-go/v7](https://github.com/minio/minio-go) | `github.com/jun3372/weaver/plugins/s3` |
| Search | [elasticsearch](/en/guide/plugins/storage#elasticsearch) | [go-elasticsearch/v9](https://github.com/elastic/go-elasticsearch) | `github.com/jun3372/weaver/plugins/elasticsearch` |
| Search | [meilisearch](/en/guide/plugins/storage#meilisearch) | [meilisearch-go](https://github.com/meilisearch/meilisearch-go) | `github.com/jun3372/weaver/plugins/meilisearch` |
| Task Scheduling | [gocron](/en/guide/plugins/scheduler#gocron) | [go-co-op/gocron/v2](https://github.com/go-co-op/gocron) | `github.com/jun3372/weaver/plugins/gocron` |

## Uniform Usage Pattern

All plugins follow the same three-step pattern, using redis as an example:

```go
import (
    "github.com/jun3372/weaver"
    _ "github.com/jun3372/weaver/plugins/redis" // blank import completes component registration
)

type sessionStore struct {
    weaver.Implements[SessionStore]
    weaver.Ref[redis.T] // declare the dependency; the framework injects it automatically
}

func (s *sessionStore) Set(ctx context.Context, key, val string) error {
    cli, err := s.Ref().Client(ctx) // obtain the underlying client
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

Each plugin's configuration has only two fields, read from the config section of the same name (e.g. `redis:`, `kafka:`):

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | Varies per plugin | Connection string in `scheme://host:port` form; see each plugin's tutorial |

Environment variables can override the config file (e.g. `WEAVER_REDIS_DSN`, `WEAVER_REDIS_ENABLED`); see [Configuration](/en/guide/config) for the rules.

### Errors

Each plugin exposes two sentinel errors via `errors.go`, which can be checked directly with `errors.Is`:

| Error | Meaning |
|---|---|
| `ErrNotEnabled` | `Enabled: false` in the configuration; the component was not initialized |
| `ErrNotReady` | Enabled but the connection is not yet ready (e.g. after a failed `Init`) |

### Lifecycle

- `Init`: if not enabled, logs and returns immediately; parses the DSN → establishes the connection → probes with `Ping`. Any failure returns an error and aborts startup
- `Shutdown`: closes the underlying client connection; safe for components that were never initialized (not enabled)

### Dependency Isolation

Each plugin has its own `go.mod`; unless imported, no client library dependency is pulled in. Plugins with `Enabled: false` (the default) never establish a connection even if imported.

## Category Tutorials

- [Database: db / mongodb](/en/guide/plugins/database)
- [Cache: redis / memcached](/en/guide/plugins/cache)
- [Messaging: kafka / rabbitmq / nats / nsq / mqtt](/en/guide/plugins/messaging)
- [Registry & Config Center: consul / etcd / nacos / zookeeper](/en/guide/plugins/registry)
- [Storage & Search: s3 / elasticsearch / meilisearch](/en/guide/plugins/storage)
- [Task Scheduling: gocron](/en/guide/plugins/scheduler)

## Further Reading

- [Configuration](/en/guide/config)
- [Lifecycle Management](/en/guide/lifecycle)
