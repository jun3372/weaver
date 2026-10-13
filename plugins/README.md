# weaver plugins

Weaver 官方客户端插件集合。每个插件是一个独立 Go module，将常用中间件客户端封装为 weaver 组件：声明 `weaver.Ref[xxx.T]` 即可注入，连接建立（`Init`）与优雅关闭（`Shutdown`）由框架生命周期托管。

## 插件列表

| 插件 | 类别 | 客户端库 | 导入路径 | 教程 |
|---|---|---|---|---|
| [db](db/) | 数据库 | [GORM](https://github.com/go-gorm/gorm)（MySQL / PostgreSQL / SQLite / ClickHouse） | `github.com/jun3372/weaver/plugins/db` | [数据库](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/database.md#db) |
| [redis](redis/) | 缓存 | [go-redis/v9](https://github.com/redis/go-redis) | `github.com/jun3372/weaver/plugins/redis` | [缓存](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/cache.md#redis) |
| [memcached](memcached/) | 缓存 | [gomemcache](https://github.com/bradfitz/gomemcache) | `github.com/jun3372/weaver/plugins/memcached` | [缓存](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/cache.md#memcached) |
| [elasticsearch](elasticsearch/) | 搜索 | [go-elasticsearch/v9](https://github.com/elastic/go-elasticsearch) | `github.com/jun3372/weaver/plugins/elasticsearch` | [存储与搜索](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/storage.md#elasticsearch) |
| [meilisearch](meilisearch/) | 搜索 | [meilisearch-go](https://github.com/meilisearch/meilisearch-go) | `github.com/jun3372/weaver/plugins/meilisearch` | [存储与搜索](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/storage.md#meilisearch) |
| [mongodb](mongodb/) | 数据库 | [mongo-go-driver](https://github.com/mongodb/mongo-go-driver) | `github.com/jun3372/weaver/plugins/mongodb` | [数据库](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/database.md#mongodb) |
| [kafka](kafka/) | 消息队列 | [sarama](https://github.com/IBM/sarama) | `github.com/jun3372/weaver/plugins/kafka` | [消息队列](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/messaging.md#kafka) |
| [rabbitmq](rabbitmq/) | 消息队列 | [amqp091-go](https://github.com/rabbitmq/amqp091-go) | `github.com/jun3372/weaver/plugins/rabbitmq` | [消息队列](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/messaging.md#rabbitmq) |
| [nats](nats/) | 消息队列 | [nats.go](https://github.com/nats-io/nats.go) | `github.com/jun3372/weaver/plugins/nats` | [消息队列](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/messaging.md#nats) |
| [nsq](nsq/) | 消息队列 | [go-nsq](https://github.com/nsqio/go-nsq) | `github.com/jun3372/weaver/plugins/nsq` | [消息队列](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/messaging.md#nsq) |
| [mqtt](mqtt/) | 消息队列 | [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) | `github.com/jun3372/weaver/plugins/mqtt` | [消息队列](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/messaging.md#mqtt) |
| [consul](consul/) | 注册/配置中心 | [consul/api](https://github.com/hashicorp/consul) | `github.com/jun3372/weaver/plugins/consul` | [注册与配置中心](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/registry.md#consul) |
| [etcd](etcd/) | 注册/配置中心 | [etcd clientv3](https://github.com/etcd-io/etcd) | `github.com/jun3372/weaver/plugins/etcd` | [注册与配置中心](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/registry.md#etcd) |
| [nacos](nacos/) | 注册/配置中心 | [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go) | `github.com/jun3372/weaver/plugins/nacos` | [注册与配置中心](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/registry.md#nacos) |
| [zookeeper](zookeeper/) | 注册/配置中心 | [go-zookeeper](https://github.com/go-zookeeper/zk) | `github.com/jun3372/weaver/plugins/zookeeper` | [注册与配置中心](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/registry.md#zookeeper) |
| [s3](s3/) | 对象存储 | [minio-go/v7](https://github.com/minio/minio-go) | `github.com/jun3372/weaver/plugins/s3` | [存储与搜索](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/storage.md#s3) |
| [gocron](gocron/) | 任务调度 | [go-co-op/gocron/v2](https://github.com/go-co-op/gocron) | `github.com/jun3372/weaver/plugins/gocron` | [任务调度](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/scheduler.md#gocron) |

## 统一使用模式

所有插件遵循相同的约定，以 db 为例：

```go
import _ "github.com/jun3372/weaver/plugins/db" // 匿名导入完成组件注册

type userRepo struct {
    weaver.Implements[UserRepo]
    weaver.Ref[db.T]
}

func (u *userRepo) Find(ctx context.Context, id int64) error {
    cli, err := u.Ref().Client(ctx) // 每个插件均提供 Client(ctx)（db 为 WithContext）
    if err != nil {
        return err // 未启用返回 ErrNotEnabled，未就绪返回 ErrNotReady
    }
    _ = cli
    return nil
}
```

### 通用约定

- **独立 module**：每个插件有自己的 `go.mod`，不使用则不引入依赖；客户端库选型优先纯 Go 实现（无 cgo），保证 `CGO_ENABLED=0` 交叉编译可用。
- **`Enabled` 开关**：配置中 `Enabled: false`（默认）时组件初始化直接跳过，注入调用返回 `ErrNotEnabled`；不使用的插件无需配置。
- **配置段**：每个插件读取同名配置段（如 `db:`、`redis:`、`kafka:`），可用环境变量覆盖（如 `WEAVER_DB_ENABLED`），见各插件 README。
- **错误**：各插件通过 `errors.go` 暴露 `ErrNotEnabled` / `ErrNotReady` 等哨兵错误，可直接 `errors.Is` 判断。
- **生命周期**：`Init` 建立连接，`Shutdown` 关闭客户端；`Shutdown` 对未初始化（未启用）的组件是安全的。

各插件的配置项、DSN/地址格式与完整示例见各自目录下的 README。
