# 注册与配置中心:consul / etcd / nacos / zookeeper

## consul

Weaver 的 Consul 客户端组件,基于官方 [hashicorp/consul/api](https://github.com/hashicorp/consul/api)。声明 `weaver.Ref[consul.T]` 即可注入 `*api.Client`,client 构建与探活由框架生命周期托管。

### 安装

```bash
go get github.com/jun3372/weaver/plugins/consul
# 主包匿名导入完成组件注册:
# import _ "github.com/jun3372/weaver/plugins/consul"
```

### 配置

```yaml
consul:
  Enabled: true
  DSN: "http://127.0.0.1:8500"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `http://127.0.0.1:8500` | Consul HTTP API 地址,scheme 仅支持 `http` / `https`;userinfo 转 Basic Auth,如 `http://admin:secret@127.0.0.1:8500` |

环境变量覆盖:`WEAVER_CONSUL_DSN`、`WEAVER_CONSUL_ENABLED`。

### 基础用法

```go
type serviceRegistry struct {
    weaver.Implements[ServiceRegistry]
    weaver.Ref[consul.T]
}

func (s *serviceRegistry) Services(ctx context.Context) (map[string][]string, error) {
    cli, err := s.Ref().Client(ctx)
    if err != nil {
        return nil, err
    }
    return cli.Catalog().Services(nil)
}
```

### 进阶场景

#### KV 读写

```go
cli, _ := s.Ref().Client(ctx)
kv := cli.KV()
_, err := kv.Put(&api.KVPair{Key: "config/feature/switch", Value: []byte("on")}, nil)
pair, _, err := kv.Get("config/feature/switch", nil)
if err == nil && pair != nil {
    fmt.Println(string(pair.Value))
}
```

#### 服务注册与健康检查

```go
err := cli.Agent().ServiceRegister(&api.AgentServiceRegistration{
    ID: "order-1", Name: "order", Address: "10.0.0.5", Port: 8080,
    Check: &api.AgentServiceCheck{
        HTTP:                           "http://10.0.0.5:8080/healthz",
        Interval:                       "10s",
        Timeout:                        "3s",
        DeregisterCriticalServiceAfter: "60s", // 持续不健康后自动摘除
    },
})
```

#### 服务发现

```go
// Health().Service 只取健康实例
entries, _, err := cli.Health().Service("order", "", true, nil)
for _, e := range entries {
    fmt.Println(e.Service.Address, e.Service.Port)
}
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;解析 DSN 填充 `api.DefaultConfig()` 后 `api.NewClient`,并 `Status().Leader()` 探活;`Shutdown` 时 HTTP client 无需显式关闭
- 未启用/未就绪时 `Client(ctx)` 返回哨兵错误 `ErrNotEnabled` / `ErrNotReady`,用 `errors.Is` 判断后走降级逻辑

## etcd

Weaver 的 etcd 客户端组件,基于官方 [etcd client/v3](https://github.com/etcd-io/etcd)。声明 `weaver.Ref[etcd.T]` 即可注入 `*clientv3.Client`,多 endpoint 建连与 `Status` 探活由框架生命周期托管。

### 安装

```bash
go get github.com/jun3372/weaver/plugins/etcd
# 主包匿名导入完成组件注册:
# import _ "github.com/jun3372/weaver/plugins/etcd"
```

### 配置

```yaml
etcd:
  Enabled: true
  DSN: "etcd://localhost:2379"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `etcd://localhost:2379` | 支持 `etcd://host1:2379,host2:2379` 多 endpoint(客户端自动负载与故障转移) |

环境变量覆盖:`WEAVER_ETCD_DSN`、`WEAVER_ETCD_ENABLED`。

### 基础用法

```go
// Get 与 Put 均通过 *clientv3.Client 直接调用
func (f *configFetcher) Get(ctx context.Context, key string) (string, error) {
    cli, _ := f.Ref().Client(ctx)
    resp, err := cli.Get(ctx, key)
    if err != nil || resp.Count == 0 { // key 不存在时 Count 为 0
        return "", err
    }
    return string(resp.Kvs[0].Value), nil
}
```

### 进阶场景

#### Put 与前缀查询

```go
cli, _ := f.Ref().Client(ctx)
cli.Put(ctx, "cfg/db/host", "10.0.0.1")
// 前缀查询
resp, _ := cli.Get(ctx, "cfg/db/", clientv3.WithPrefix())
for _, kv := range resp.Kvs {
    fmt.Printf("%s = %s\n", kv.Key, kv.Value)
}
```

#### Watch 变更监听

```go
// Watch 返回 clientv3.WatchChan,持续消费变更事件
ch := cli.Watch(ctx, "cfg/db/", clientv3.WithPrefix())
for resp := range ch {
    for _, ev := range resp.Events {
        if ev.Type == clientv3.EventTypePut {
            fmt.Printf("更新 %s = %s\n", ev.Kv.Key, ev.Kv.Value)
        } else { // EventTypeDelete
            fmt.Printf("删除 %s\n", ev.Kv.Key)
        }
    }
}
```

#### Lease 租约保活

```go
grant, _ := cli.Grant(ctx, 15) // 15s TTL
cli.Put(ctx, "nodes/node-1", "10.0.0.5", clientv3.WithLease(grant.ID))
ka, _ := cli.KeepAlive(ctx, grant.ID)
go func() {
    for range ka { // 消费续期响应即可,客户端自动续期
    }
}()
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;解析 DSN 得到 endpoint 列表后 `clientv3.New`(DialTimeout 5s),并对首个 endpoint `Status` 探活,失败自动 `Close`;`Shutdown` 时 `Close` 关闭客户端
- 常见错误:`ErrNotEnabled`/`ErrNotReady`(插件未启用/未就绪)、`ErrInvalidDSN`(scheme 不是 `etcd` 或无可用 endpoint)

## nacos

Weaver 的 Nacos 客户端组件,基于官方 [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go/v2)。声明 `weaver.Ref[nacos.T]` 后可分别注入服务发现客户端 `NamingClient` 与配置中心客户端 `ConfigClient`,两者共用同一份 DSN 配置。

### 安装

```bash
go get github.com/jun3372/weaver/plugins/nacos
# 主包匿名导入完成组件注册:
# import _ "github.com/jun3372/weaver/plugins/nacos"
```

### 配置

```yaml
nacos:
  Enabled: true
  DSN: "nacos://localhost:8848"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `nacos://localhost:8848` | 支持 `nacos://user:pass@host1:8848,host2:8848?namespaceId=xxx` 形式:userinfo 转 username/password,缺省端口 8848,查询参数指定 namespace |

环境变量覆盖:`WEAVER_NACOS_DSN`、`WEAVER_NACOS_ENABLED`。

### 基础用法

> 获取客户端的方法是 `NamingClient(ctx)` 与 `ConfigClient(ctx)`,不要写成 `Naming(ctx)` / `Config(ctx)`。

```go
type serviceRegistry struct {
    weaver.Implements[ServiceRegistry]
    weaver.Ref[nacos.T]
}

// 服务发现:走 NamingClient
func (s *serviceRegistry) Instances(ctx context.Context, service string) ([]vo.Instance, error) {
    cli, _ := s.Ref().NamingClient(ctx)
    return cli.SelectInstances(vo.SelectAllInstancesParam{
        ServiceName: service, GroupName: "DEFAULT_GROUP",
    })
}

// 配置中心:走 ConfigClient
func (s *serviceRegistry) LoadConfig(ctx context.Context, dataID string) (string, error) {
    cli, _ := s.Ref().ConfigClient(ctx)
    return cli.GetConfig(vo.ConfigParam{DataId: dataID, Group: "DEFAULT_GROUP"})
}
```

### 进阶场景

#### 服务注册与注销

```go
naming, _ := s.Ref().NamingClient(ctx)
// 注册实例:临时实例,客户端断开后自动摘除
_ = naming.RegisterInstance(vo.RegisterInstanceParam{
    Ip: "10.0.0.5", Port: 8080, ServiceName: "order", GroupName: "DEFAULT_GROUP",
    Weight: 10, Enable: true, Healthy: true, Ephemeral: true,
})
// 注销实例
_ = naming.DeregisterInstance(vo.DeregisterInstanceParam{
    Ip: "10.0.0.5", Port: 8080, ServiceName: "order", GroupName: "DEFAULT_GROUP",
})
```

#### 配置监听

```go
config, _ := s.Ref().ConfigClient(ctx)
_ = config.ListenConfig(vo.ListenParam{
    DataId: "order.yaml", Group: "DEFAULT_GROUP",
    OnChange: func(namespace, group, dataId, data string) {
        fmt.Printf("配置变更:%s = %s\n", dataId, data)
    },
})
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;解析 DSN 构建共享的 `ClientConfig` 与 `ServerConfigs` 后依次创建两个客户端,后者创建失败时回滚前者;随后拉取服务列表探活(空列表也算成功);`Shutdown` 时依次 `CloseClient` 关闭两者
- 常见错误:`ErrNotEnabled`/`ErrNotReady`(插件未启用/未就绪)、`ErrInvalidDSN`、`ErrInvalidScheme`

## zookeeper

Weaver 的 ZooKeeper 客户端组件,基于 [go-zookeeper/zk](https://github.com/go-zookeeper/zk)。声明 `weaver.Ref[zookeeper.T]` 即可注入 `*zk.Conn`,建连与 `StateConnected` 探活由框架生命周期托管。

### 安装

```bash
go get github.com/jun3372/weaver/plugins/zookeeper

# 主包匿名导入完成组件注册:
# import _ "github.com/jun3372/weaver/plugins/zookeeper"
```

### 配置

```yaml
zookeeper:
  Enabled: true
  DSN: "zk://localhost:2181"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `DSN` | `zk://localhost:2181` | 支持 `zk://host1:2181,host2:2181` 多 host 集群地址 |

环境变量覆盖:`WEAVER_ZOOKEEPER_DSN`、`WEAVER_ZOOKEEPER_ENABLED`。

### 基础用法

```go
// zk.Conn 的 Get 返回数据与 Stat
func (l *lockRepo) Get(ctx context.Context, path string) ([]byte, error) {
    conn, err := l.Ref().Client(ctx)
    if err != nil {
        return nil, err
    }
    data, _, err := conn.Get(path)
    return data, err
}
```

### 进阶场景

#### 节点创建与 flag 常量

```go
conn, _ := l.Ref().Client(ctx)
// 持久节点(flag 为 0)
conn.Create("/weaver", []byte{}, 0, zk.WorldACL(zk.PermAll))
// 临时节点:会话断开自动删除,常用于服务注册
conn.Create("/weaver/nodes/node-1", []byte("10.0.0.5:8080"),
    zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
// 临时顺序节点:分布式锁的经典选法
conn.Create("/weaver/locks/order-", []byte{},
    zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
```

#### Set 与版本控制

```go
// Get 返回 Stat,Set 带 version 做乐观锁(version 传 -1 表示不校验)
data, stat, _ := conn.Get("/weaver/config")
_, _ = conn.Set("/weaver/config", []byte("new-value"), stat.Version)
```

#### 存在监听(ExistsW)

```go
exists, _, ch, _ := conn.ExistsW("/weaver/nodes/node-1")
if exists {
    ev := <-ch
    if ev.Type == zk.EventNodeDeleted {
        fmt.Println("节点已下线:", ev.Path)
    }
}
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;解析 DSN 得到 host 列表后 `zk.Connect`(session timeout 5s,内部自动重试建连),若未进入 `StateConnected` 状态则 `Close` 并返回 `ErrNotReady`;`Shutdown` 时 `Close` 关闭连接
- 常见错误:`zk.ErrNodeExists`(节点已存在)、`zk.ErrNoNode`(节点不存在)、`ErrNotEnabled`/`ErrNotReady`(插件未启用/未就绪)、`ErrInvalidDSN`

## 相关阅读

- [插件总览](/guide/plugins)
- [配置管理](/guide/config)
