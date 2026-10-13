# 插件:中间件客户端集成

`plugins/` 目录提供 15 个官方维护的中间件客户端插件。每个插件是一个独立 Go module,把常用中间件的客户端封装为 weaver 组件:声明 `weaver.Ref[xxx.T]` 即可注入,连接建立(`Init`)与优雅关闭(`Shutdown`)由框架生命周期托管。所有客户端库均为纯 Go 实现,`CGO_ENABLED=0` 交叉编译可用。

## 插件列表

| 类别 | 插件 | 客户端库 | 导入路径 |
|---|---|---|---|
| 数据库 | [db](/guide/plugins/database#db) | [GORM](https://github.com/go-gorm/gorm) | `github.com/jun3372/weaver/plugins/db` |
| 数据库 | [mongodb](/guide/plugins/database#mongodb) | [mongo-driver/v2](https://github.com/mongodb/mongo-go-driver) | `github.com/jun3372/weaver/plugins/mongodb` |
| 缓存 | [redis](/guide/plugins/cache#redis) | [go-redis/v9](https://github.com/redis/go-redis) | `github.com/jun3372/weaver/plugins/redis` |
| 缓存 | [memcached](/guide/plugins/cache#memcached) | [gomemcache](https://github.com/bradfitz/gomemcache) | `github.com/jun3372/weaver/plugins/memcached` |
| 消息队列 | [kafka](/guide/plugins/messaging#kafka) | [sarama](https://github.com/IBM/sarama) | `github.com/jun3372/weaver/plugins/kafka` |
| 消息队列 | [rabbitmq](/guide/plugins/messaging#rabbitmq) | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | `github.com/jun3372/weaver/plugins/rabbitmq` |
| 消息队列 | [nats](/guide/plugins/messaging#nats) | [nats.go](https://github.com/nats-io/nats.go) | `github.com/jun3372/weaver/plugins/nats` |
| 消息队列 | [nsq](/guide/plugins/messaging#nsq) | [go-nsq](https://github.com/nsqio/go-nsq) | `github.com/jun3372/weaver/plugins/nsq` |
| 消息队列 | [mqtt](/guide/plugins/messaging#mqtt) | [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) | `github.com/jun3372/weaver/plugins/mqtt` |
| 注册/配置中心 | [consul](/guide/plugins/registry#consul) | [consul/api](https://github.com/hashicorp/consul) | `github.com/jun3372/weaver/plugins/consul` |
| 注册/配置中心 | [etcd](/guide/plugins/registry#etcd) | [etcd clientv3](https://github.com/etcd-io/etcd) | `github.com/jun3372/weaver/plugins/etcd` |
| 注册/配置中心 | [nacos](/guide/plugins/registry#nacos) | [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go) | `github.com/jun3372/weaver/plugins/nacos` |
| 注册/配置中心 | [zookeeper](/guide/plugins/registry#zookeeper) | [go-zookeeper](https://github.com/go-zookeeper/zk) | `github.com/jun3372/weaver/plugins/zookeeper` |
| 对象存储 | [s3](/guide/plugins/storage#s3) | [minio-go/v7](https://github.com/minio/minio-go) | `github.com/jun3372/weaver/plugins/s3` |
| 搜索 | [elasticsearch](/guide/plugins/storage#elasticsearch) | [go-elasticsearch/v9](https://github.com/elastic/go-elasticsearch) | `github.com/jun3372/weaver/plugins/elasticsearch` |
| 搜索 | [meilisearch](/guide/plugins/storage#meilisearch) | [meilisearch-go](https://github.com/meilisearch/meilisearch-go) | `github.com/jun3372/weaver/plugins/meilisearch` |

## 统一使用模式

所有插件遵循相同的三步模式,以 redis 为例:

```go
import (
    "github.com/jun3372/weaver"
    _ "github.com/jun3372/weaver/plugins/redis" // 匿名导入完成组件注册
)

type sessionStore struct {
    weaver.Implements[SessionStore]
    weaver.Ref[redis.T] // 声明依赖,框架自动注入
}

func (s *sessionStore) Set(ctx context.Context, key, val string) error {
    cli, err := s.Ref().Client(ctx) // 获取底层客户端
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

每个插件的配置只有两个字段,读取同名配置段(如 `redis:`、`kafka:`):

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | 各插件不同 | 连接串,`scheme://host:port` 形式,见各插件教程 |

环境变量可覆盖配置文件(如 `WEAVER_REDIS_DSN`、`WEAVER_REDIS_ENABLED`),规则见[配置管理](/guide/config)。

### 错误

各插件通过 `errors.go` 暴露两个哨兵错误,可直接 `errors.Is` 判断:

| 错误 | 含义 |
|---|---|
| `ErrNotEnabled` | 配置中 `Enabled: false`,组件未初始化 |
| `ErrNotReady` | 已启用但连接尚未就绪(如 `Init` 失败后) |

### 生命周期

- `Init`:未启用时打日志直接返回;解析 DSN → 建立连接 → `Ping` 探活,任一失败即返回错误中止启动
- `Shutdown`:关闭底层客户端连接;对未初始化(未启用)的组件是安全的

### 依赖隔离

每个插件有独立的 `go.mod`,不导入则不引入任何客户端库依赖;`Enabled: false`(默认)的插件即使被导入也不会建立连接。

## 分类教程

- [数据库:db / mongodb](/guide/plugins/database)
- [缓存:redis / memcached](/guide/plugins/cache)
- [消息队列:kafka / rabbitmq / nats / nsq / mqtt](/guide/plugins/messaging)
- [注册与配置中心:consul / etcd / nacos / zookeeper](/guide/plugins/registry)
- [存储与搜索:s3 / elasticsearch / meilisearch](/guide/plugins/storage)

## 相关阅读

- [配置管理](/guide/config)
- [生命周期管理](/guide/lifecycle)
