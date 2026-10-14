# 组件:中间件客户端集成

`components/` 目录提供 17 个官方维护的中间件客户端组件。每个组件是一个独立 Go module,把常用中间件的客户端封装为 weaver 组件:声明 `weaver.Ref[xxx.T]` 即可注入,连接建立(`Init`)与优雅关闭(`Shutdown`)由框架生命周期托管。所有客户端库均为纯 Go 实现,`CGO_ENABLED=0` 交叉编译可用。

## 组件列表

| 类别 | 组件 | 客户端库 | 导入路径 |
|---|---|---|---|
| 数据库 | [db](/guide/components/database#db) | [GORM](https://github.com/go-gorm/gorm) | `github.com/jun3372/weaver/components/db` |
| 数据库 | [mongodb](/guide/components/database#mongodb) | [mongo-driver/v2](https://github.com/mongodb/mongo-go-driver) | `github.com/jun3372/weaver/components/mongodb` |
| 缓存 | [redis](/guide/components/cache#redis) | [go-redis/v9](https://github.com/redis/go-redis) | `github.com/jun3372/weaver/components/redis` |
| 缓存 | [memcached](/guide/components/cache#memcached) | [gomemcache](https://github.com/bradfitz/gomemcache) | `github.com/jun3372/weaver/components/memcached` |
| 消息队列 | [kafka](/guide/components/messaging#kafka) | [sarama](https://github.com/IBM/sarama) | `github.com/jun3372/weaver/components/kafka` |
| 消息队列 | [rabbitmq](/guide/components/messaging#rabbitmq) | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | `github.com/jun3372/weaver/components/rabbitmq` |
| 消息队列 | [nats](/guide/components/messaging#nats) | [nats.go](https://github.com/nats-io/nats.go) | `github.com/jun3372/weaver/components/nats` |
| 消息队列 | [nsq](/guide/components/messaging#nsq) | [go-nsq](https://github.com/nsqio/go-nsq) | `github.com/jun3372/weaver/components/nsq` |
| 消息队列 | [mqtt](/guide/components/messaging#mqtt) | [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) | `github.com/jun3372/weaver/components/mqtt` |
| 注册/配置中心 | [consul](/guide/components/registry#consul) | [consul/api](https://github.com/hashicorp/consul) | `github.com/jun3372/weaver/components/consul` |
| 注册/配置中心 | [etcd](/guide/components/registry#etcd) | [etcd clientv3](https://github.com/etcd-io/etcd) | `github.com/jun3372/weaver/components/etcd` |
| 注册/配置中心 | [nacos](/guide/components/registry#nacos) | [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go) | `github.com/jun3372/weaver/components/nacos` |
| 注册/配置中心 | [zookeeper](/guide/components/registry#zookeeper) | [go-zookeeper](https://github.com/go-zookeeper/zk) | `github.com/jun3372/weaver/components/zookeeper` |
| 对象存储 | [s3](/guide/components/storage#s3) | [minio-go/v7](https://github.com/minio/minio-go) | `github.com/jun3372/weaver/components/s3` |
| 搜索 | [elasticsearch](/guide/components/storage#elasticsearch) | [go-elasticsearch/v9](https://github.com/elastic/go-elasticsearch) | `github.com/jun3372/weaver/components/elasticsearch` |
| 搜索 | [meilisearch](/guide/components/storage#meilisearch) | [meilisearch-go](https://github.com/meilisearch/meilisearch-go) | `github.com/jun3372/weaver/components/meilisearch` |
| 任务调度 | [gocron](/guide/components/scheduler#gocron) | [go-co-op/gocron/v2](https://github.com/go-co-op/gocron) | `github.com/jun3372/weaver/components/gocron` |

## 统一使用模式

所有组件遵循相同的三步模式,以 redis 为例:

```go
import (
    "github.com/jun3372/weaver"
    _ "github.com/jun3372/weaver/components/redis" // 匿名导入完成组件注册
)

type sessionStore struct {
    weaver.Implements[SessionStore]
    weaver.Ref[redis.T] // 声明依赖,框架自动注入
}

func (s *sessionStore) Set(ctx context.Context, key, val string) error {
    cli, err := s.Get().Client(ctx) // 获取底层客户端
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

## 通用约定

### 配置

每个组件的配置只有两个字段,读取同名配置段(如 `redis:`、`kafka:`):

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | 各组件不同 | 连接串,`scheme://host:port` 形式,见各组件教程 |

环境变量可覆盖配置文件(如 `WEAVER_REDIS_DSN`、`WEAVER_REDIS_ENABLED`),规则见[配置管理](/guide/config)。

### 错误

各组件通过 `errors.go` 暴露两个哨兵错误,可直接 `errors.Is` 判断:

| 错误 | 含义 |
|---|---|
| `ErrNotEnabled` | 配置中 `Enabled: false`,组件未初始化 |
| `ErrNotReady` | 已启用但连接尚未就绪(如 `Init` 失败后) |

### 生命周期

- `Init`:未启用时打日志直接返回;解析 DSN → 建立连接 → `Ping` 探活,任一失败即返回错误中止启动
- `Shutdown`:关闭底层客户端连接;对未初始化(未启用)的组件是安全的

### 依赖隔离

每个组件有独立的 `go.mod`,不导入则不引入任何客户端库依赖;`Enabled: false`(默认)的组件即使被导入也不会建立连接。

## 分类教程

- [数据库:db / mongodb](/guide/components/database)
- [缓存:redis / memcached](/guide/components/cache)
- [消息队列:kafka / rabbitmq / nats / nsq / mqtt](/guide/components/messaging)
- [注册与配置中心:consul / etcd / nacos / zookeeper](/guide/components/registry)
- [存储与搜索:s3 / elasticsearch / meilisearch](/guide/components/storage)
- [任务调度:gocron](/guide/components/scheduler)

## 相关阅读

- [配置管理](/guide/config)
- [生命周期管理](/guide/lifecycle)
