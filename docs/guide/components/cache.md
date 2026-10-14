# 缓存:redis / memcached

## redis

Weaver 的 Redis 客户端组件,基于 [go-redis/v9](https://github.com/redis/go-redis)。声明 `weaver.Ref[redis.T]` 即可注入 `*redis.Client`,连接建立、Ping 探活与优雅关闭由框架生命周期托管。

### 安装

```bash
go get github.com/jun3372/weaver/components/redis
```

```go
import _ "github.com/jun3372/weaver/components/redis" // 匿名导入完成组件注册
```

### 配置

```yaml
redis:
  Enabled: true
  DSN: "redis://localhost:6379/0"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `redis://localhost:6379/0` | Redis URL,支持 `redis://user:pass@host:port/db` 与 `rediss://`(TLS) |

环境变量覆盖:`WEAVER_REDIS_DSN`、`WEAVER_REDIS_ENABLED`。

### 基础用法

```go
type sessionStore struct {
    weaver.Implements[SessionStore]
    weaver.Ref[redis.T]
}

func (s *sessionStore) Set(ctx context.Context, key, val string) error {
    cli, err := s.Get().Client(ctx)
    if err != nil {
        return err
    }
    return cli.Set(ctx, key, val, time.Hour).Err()
}

func (s *sessionStore) Get(ctx context.Context, key string) (string, error) {
    cli, err := s.Get().DB(ctx) // DB 与 Client 等价
    if err != nil {
        return "", err
    }
    return cli.Get(ctx, key).Result()
}
```

### 进阶场景

#### 常用数据结构

```go
cli, _ := s.Get().Client(ctx)

// Hash
cli.HSet(ctx, "user:1", "name", "alice", "age", 20)
cli.HGet(ctx, "user:1", "name")

// List
cli.RPush(ctx, "queue:mail", "task-1")

// Stream(消费组)
cli.XGroupCreateMkStream(ctx, "orders", "workers", "0")
msgs, _ := cli.XReadGroup(ctx, &redis.XReadGroupArgs{
    Group:    "workers",
    Consumer: "worker-1",
    Streams:  []string{"orders", ">"},
})

// Set + 过期(验证码场景)
cli.Set(ctx, "captcha:abc", "123456", 5*time.Minute)
```

#### Pipeline 与 Pub/Sub

```go
// Pipeline:批量命令一次往返
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

#### 客户端复用

`Client(ctx)` 返回的是组件初始化时创建的同一个 `*redis.Client`(go-redis 内部维护连接池,并发安全)。**在组件内复用即可,不要每次调用都获取后丢弃**——正确做法是把 `*redis.Client` 存入组件字段或直接在方法内按需获取。

### 生命周期与错误

- `Init`:未启用时打日志直接返回;`redis.ParseURL` 解析 DSN 后 `redis.NewClient`,并 `Ping` 探活,失败时自动 `Close` 并返回错误
- `Shutdown`:`Close` 关闭客户端

```go
cli, err := s.Get().Client(ctx)
if errors.Is(err, redis.ErrNotEnabled) {
    // redis 组件未启用,走降级逻辑
}
```

## memcached

Weaver 的 Memcached 客户端组件,基于 [gomemcache](https://github.com/bradfitz/gomemcache)。声明 `weaver.Ref[memcached.T]` 即可注入 `*memcache.Client`。

### 安装

```bash
go get github.com/jun3372/weaver/components/memcached
```

```go
import _ "github.com/jun3372/weaver/components/memcached"
```

### 配置

```yaml
memcached:
  Enabled: true
  DSN: "memcache://localhost:11211"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `memcache://localhost:11211` | 支持 `memcache://host1:11211,host2:11211` 多节点(客户端自动做一致性哈希分片) |

环境变量覆盖:`WEAVER_MEMCACHED_DSN`、`WEAVER_MEMCACHED_ENABLED`。

### 基础用法

```go
type cacheRepo struct {
    weaver.Implements[CacheRepo]
    weaver.Ref[memcached.T]
}

func (c *cacheRepo) Set(ctx context.Context, key string, val []byte) error {
    cli, err := c.Get().Client(ctx)
    if err != nil {
        return err
    }
    return cli.Set(&memcache.Item{Key: key, Value: val, Expiration: 3600})
}

func (c *cacheRepo) Get(ctx context.Context, key string) ([]byte, error) {
    cli, err := c.Get().Client(ctx)
    if err != nil {
        return nil, err
    }
    item, err := cli.Get(key)
    if errors.Is(err, memcache.ErrCacheMiss) {
        return nil, nil // 未命中
    }
    if err != nil {
        return nil, err
    }
    return item.Value, nil
}
```

### 进阶场景

#### 原子计数器

```go
cli, _ := c.Get().Client(ctx)

cli.Add(&memcache.Item{Key: "hits", Value: []byte("0")}) // 不存在才初始化
_, _ = cli.Increment("hits", 1)                          // 原子自增
_, _ = cli.Decrement("hits", 1)                          // 原子自减
```

#### GetMulti 批量读取

```go
items, err := cli.GetMulti([]string{"k1", "k2", "k3"})
// 返回 map[string]*memcache.Item,缺失的 key 不在 map 中
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;按 DSN 解析多节点地址后 `memcache.New` 并 `Ping` 探活
- `Shutdown`:gomemcache 客户端内部管理连接,无需显式关闭
- 常见错误:`memcache.ErrCacheMiss`(key 不存在)、`ErrNotEnabled`/`ErrNotReady`(组件未启用/未就绪)

## 相关阅读

- [组件总览](/guide/components)
- [配置管理](/guide/config)
