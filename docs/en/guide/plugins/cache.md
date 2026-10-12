# Cache: redis / memcached

## redis

Weaver's Redis client component, built on [go-redis/v9](https://github.com/redis/go-redis). Declare a `weaver.Ref[redis.T]` to inject a `*redis.Client`; connection establishment, the `Ping` health check and graceful shutdown are managed by the framework lifecycle.

### Installation

```bash
go get github.com/jun3372/weaver/plugins/redis
```

```go
import _ "github.com/jun3372/weaver/plugins/redis" // blank import completes component registration
```

### Configuration

```yaml
redis:
  Enabled: true
  DSN: "redis://localhost:6379/0"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `redis://localhost:6379/0` | Redis URL; supports `redis://user:pass@host:port/db` and `rediss://` (TLS) |

Environment variable overrides: `WEAVER_REDIS_DSN`, `WEAVER_REDIS_ENABLED`.

### Basic Usage

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

func (s *sessionStore) Get(ctx context.Context, key string) (string, error) {
    cli, err := s.Ref().DB(ctx) // DB is equivalent to Client
    if err != nil {
        return "", err
    }
    return cli.Get(ctx, key).Result()
}
```

### Advanced Scenarios

#### Common Data Structures

```go
cli, _ := s.Ref().Client(ctx)

// Hash
cli.HSet(ctx, "user:1", "name", "alice", "age", 20)
cli.HGet(ctx, "user:1", "name")

// List
cli.RPush(ctx, "queue:mail", "task-1")

// Stream (consumer group)
cli.XGroupCreateMkStream(ctx, "orders", "workers", "0")
msgs, _ := cli.XReadGroup(ctx, &redis.XReadGroupArgs{
    Group:    "workers",
    Consumer: "worker-1",
    Streams:  []string{"orders", ">"},
})

// Set + expiration (captcha scenario)
cli.Set(ctx, "captcha:abc", "123456", 5*time.Minute)
```

#### Pipeline and Pub/Sub

```go
// Pipeline: batch commands in a single round trip
pipe := cli.Pipeline()
for i := 0; i < 100; i++ {
    pipe.Set(ctx, fmt.Sprintf("k:%d", i), i, time.Hour)
}
_, _ = pipe.Exec(ctx)

// Pub/Sub
sub := cli.Subscribe(ctx, "events")
defer sub.Close()
go func() {
    for msg := range sub.Channel() {
        fmt.Println(msg.Channel, msg.Payload)
    }
}()
cli.Publish(ctx, "events", "hello")
```

#### Client Reuse

`Client(ctx)` returns the same `*redis.Client` created when the component was initialized (go-redis maintains an internal connection pool and is concurrency-safe). **Reuse it within the component; do not fetch and discard it on every call** — the right approach is to store the `*redis.Client` in a component field, or fetch it on demand inside a method.

### Lifecycle & Errors

- `Init`: if not enabled, logs and returns immediately; parses the DSN with `redis.ParseURL`, creates the client with `redis.NewClient`, and probes with `Ping`. On failure the client is closed automatically and an error is returned
- `Shutdown`: `Close` closes the client

```go
cli, err := s.Ref().Client(ctx)
if errors.Is(err, redis.ErrNotEnabled) {
    // redis plugin not enabled; fall back to degraded logic
}
```

## memcached

Weaver's Memcached client component, built on [gomemcache](https://github.com/bradfitz/gomemcache). Declare a `weaver.Ref[memcached.T]` to inject a `*memcache.Client`.

### Installation

```bash
go get github.com/jun3372/weaver/plugins/memcached
```

```go
import _ "github.com/jun3372/weaver/plugins/memcached"
```

### Configuration

```yaml
memcached:
  Enabled: true
  DSN: "memcache://localhost:11211"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `memcache://localhost:11211` | Supports multi-node `memcache://host1:11211,host2:11211` (the client shards automatically via consistent hashing) |

Environment variable overrides: `WEAVER_MEMCACHED_DSN`, `WEAVER_MEMCACHED_ENABLED`.

### Basic Usage

```go
type cacheRepo struct {
    weaver.Implements[CacheRepo]
    weaver.Ref[memcached.T]
}

func (c *cacheRepo) Set(ctx context.Context, key string, val []byte) error {
    cli, err := c.Ref().Client(ctx)
    if err != nil {
        return err
    }
    return cli.Set(&memcache.Item{Key: key, Value: val, Expiration: 3600})
}

func (c *cacheRepo) Get(ctx context.Context, key string) ([]byte, error) {
    cli, err := c.Ref().Client(ctx)
    if err != nil {
        return nil, err
    }
    item, err := cli.Get(key)
    if errors.Is(err, memcache.ErrCacheMiss) {
        return nil, nil // cache miss
    }
    if err != nil {
        return nil, err
    }
    return item.Value, nil
}
```

### Advanced Scenarios

#### Atomic Counters

```go
cli, _ := c.Ref().Client(ctx)

cli.Add(&memcache.Item{Key: "hits", Value: []byte("0")}) // initialize only if absent
_, _ = cli.Increment("hits", 1)                          // atomic increment
_, _ = cli.Decrement("hits", 1)                          // atomic decrement
```

#### GetMulti Batch Reads

```go
items, err := cli.GetMulti([]string{"k1", "k2", "k3"})
// Returns map[string]*memcache.Item; missing keys are absent from the map
```

### Lifecycle & Errors

- `Init`: if not enabled, logs and returns immediately; parses the multi-node addresses from the DSN, creates the client with `memcache.New` and probes with `Ping`
- `Shutdown`: the gomemcache client manages connections internally; no explicit close needed
- Common errors: `memcache.ErrCacheMiss` (key not found), `ErrNotEnabled`/`ErrNotReady` (plugin not enabled / not ready)

## Further Reading

- [Plugins Overview](/en/guide/plugins)
- [Configuration](/en/guide/config)
