# Registry & Config Center: consul / etcd / nacos / zookeeper

## consul

The Consul client component for Weaver, based on the official [hashicorp/consul/api](https://github.com/hashicorp/consul/api). Simply declare `weaver.Ref[consul.T]` to inject an `*api.Client`; client construction and liveness probing are managed by the framework lifecycle.

### Installation

```bash
go get github.com/jun3372/weaver/plugins/consul
# Anonymous import of the main package completes component registration:
# import _ "github.com/jun3372/weaver/plugins/consul"
```

### Configuration

```yaml
consul:
  Enabled: true
  DSN: "http://127.0.0.1:8500"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `http://127.0.0.1:8500` | Consul HTTP API address; scheme only supports `http` / `https`; userinfo is converted to Basic Auth, e.g. `http://admin:secret@127.0.0.1:8500` |

Environment variable overrides: `WEAVER_CONSUL_DSN`, `WEAVER_CONSUL_ENABLED`.

### Basic Usage

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

### Advanced Scenarios

#### KV Read & Write

```go
cli, _ := s.Ref().Client(ctx)
kv := cli.KV()
_, err := kv.Put(&api.KVPair{Key: "config/feature/switch", Value: []byte("on")}, nil)
pair, _, err := kv.Get("config/feature/switch", nil)
if err == nil && pair != nil {
    fmt.Println(string(pair.Value))
}
```

#### Service Registration & Health Checks

```go
err := cli.Agent().ServiceRegister(&api.AgentServiceRegistration{
    ID: "order-1", Name: "order", Address: "10.0.0.5", Port: 8080,
    Check: &api.AgentServiceCheck{
        HTTP:                           "http://10.0.0.5:8080/healthz",
        Interval:                       "10s",
        Timeout:                        "3s",
        DeregisterCriticalServiceAfter: "60s", // automatically deregistered after staying unhealthy
    },
})
```

#### Service Discovery

```go
// Health().Service only returns healthy instances
entries, _, err := cli.Health().Service("order", "", true, nil)
for _, e := range entries {
    fmt.Println(e.Service.Address, e.Service.Port)
}
```

### Lifecycle & Errors

- `Init`: logs and returns immediately when disabled; parses the DSN into `api.DefaultConfig()`, then calls `api.NewClient` and probes liveness via `Status().Leader()`; on `Shutdown`, the HTTP client needs no explicit close
- When disabled/not ready, `Client(ctx)` returns the sentinel errors `ErrNotEnabled` / `ErrNotReady`; check them with `errors.Is` and fall back to degraded logic

## etcd

The etcd client component for Weaver, based on the official [etcd client/v3](https://github.com/etcd-io/etcd). Simply declare `weaver.Ref[etcd.T]` to inject a `*clientv3.Client`; multi-endpoint connection setup and `Status` liveness probing are managed by the framework lifecycle.

### Installation

```bash
go get github.com/jun3372/weaver/plugins/etcd
# Anonymous import of the main package completes component registration:
# import _ "github.com/jun3372/weaver/plugins/etcd"
```

### Configuration

```yaml
etcd:
  Enabled: true
  DSN: "etcd://localhost:2379"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `etcd://localhost:2379` | Supports multiple endpoints such as `etcd://host1:2379,host2:2379` (the client handles load balancing and failover automatically) |

Environment variable overrides: `WEAVER_ETCD_DSN`, `WEAVER_ETCD_ENABLED`.

### Basic Usage

```go
// Both Get and Put are called directly through *clientv3.Client
func (f *configFetcher) Get(ctx context.Context, key string) (string, error) {
    cli, _ := f.Ref().Client(ctx)
    resp, err := cli.Get(ctx, key)
    if err != nil || resp.Count == 0 { // Count is 0 when the key does not exist
        return "", err
    }
    return string(resp.Kvs[0].Value), nil
}
```

### Advanced Scenarios

#### Put and Prefix Queries

```go
cli, _ := f.Ref().Client(ctx)
cli.Put(ctx, "cfg/db/host", "10.0.0.1")
// prefix query
resp, _ := cli.Get(ctx, "cfg/db/", clientv3.WithPrefix())
for _, kv := range resp.Kvs {
    fmt.Printf("%s = %s\n", kv.Key, kv.Value)
}
```

#### Watch for Change Events

```go
// Watch returns a clientv3.WatchChan; consume change events continuously
ch := cli.Watch(ctx, "cfg/db/", clientv3.WithPrefix())
for resp := range ch {
    for _, ev := range resp.Events {
        if ev.Type == clientv3.EventTypePut {
            fmt.Printf("updated %s = %s\n", ev.Kv.Key, ev.Kv.Value)
        } else { // EventTypeDelete
            fmt.Printf("deleted %s\n", ev.Kv.Key)
        }
    }
}
```

#### Lease Keep-Alive

```go
grant, _ := cli.Grant(ctx, 15) // 15s TTL
cli.Put(ctx, "nodes/node-1", "10.0.0.5", clientv3.WithLease(grant.ID))
ka, _ := cli.KeepAlive(ctx, grant.ID)
go func() {
    for range ka { // just consume the keep-alive responses; the client renews automatically
    }
}()
```

### Lifecycle & Errors

- `Init`: logs and returns immediately when disabled; parses the DSN into an endpoint list, then calls `clientv3.New` (DialTimeout 5s) and probes the first endpoint via `Status`, closing the client automatically on failure; on `Shutdown`, `Close` closes the client
- Common errors: `ErrNotEnabled`/`ErrNotReady` (plugin disabled/not ready), `ErrInvalidDSN` (scheme is not `etcd` or no usable endpoint)

## nacos

The Nacos client component for Weaver, based on the official [nacos-sdk-go/v2](https://github.com/nacos-group/nacos-sdk-go/v2). After declaring `weaver.Ref[nacos.T]`, you can inject the service discovery client `NamingClient` and the config center client `ConfigClient` separately; both share the same DSN configuration.

### Installation

```bash
go get github.com/jun3372/weaver/plugins/nacos
# Anonymous import of the main package completes component registration:
# import _ "github.com/jun3372/weaver/plugins/nacos"
```

### Configuration

```yaml
nacos:
  Enabled: true
  DSN: "nacos://localhost:8848"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `nacos://localhost:8848` | Supports the form `nacos://user:pass@host1:8848,host2:8848?namespaceId=xxx`: userinfo becomes username/password, the default port is 8848, and query parameters specify the namespace |

Environment variable overrides: `WEAVER_NACOS_DSN`, `WEAVER_NACOS_ENABLED`.

### Basic Usage

> The methods for obtaining clients are `NamingClient(ctx)` and `ConfigClient(ctx)`; do not write `Naming(ctx)` / `Config(ctx)`.

```go
type serviceRegistry struct {
    weaver.Implements[ServiceRegistry]
    weaver.Ref[nacos.T]
}

// Service discovery: via NamingClient
func (s *serviceRegistry) Instances(ctx context.Context, service string) ([]vo.Instance, error) {
    cli, _ := s.Ref().NamingClient(ctx)
    return cli.SelectInstances(vo.SelectAllInstancesParam{
        ServiceName: service, GroupName: "DEFAULT_GROUP",
    })
}

// Config center: via ConfigClient
func (s *serviceRegistry) LoadConfig(ctx context.Context, dataID string) (string, error) {
    cli, _ := s.Ref().ConfigClient(ctx)
    return cli.GetConfig(vo.ConfigParam{DataId: dataID, Group: "DEFAULT_GROUP"})
}
```

### Advanced Scenarios

#### Service Registration & Deregistration

```go
naming, _ := s.Ref().NamingClient(ctx)
// register instance: ephemeral instance, automatically removed after the client disconnects
_ = naming.RegisterInstance(vo.RegisterInstanceParam{
    Ip: "10.0.0.5", Port: 8080, ServiceName: "order", GroupName: "DEFAULT_GROUP",
    Weight: 10, Enable: true, Healthy: true, Ephemeral: true,
})
// deregister instance
_ = naming.DeregisterInstance(vo.DeregisterInstanceParam{
    Ip: "10.0.0.5", Port: 8080, ServiceName: "order", GroupName: "DEFAULT_GROUP",
})
```

#### Config Listening

```go
config, _ := s.Ref().ConfigClient(ctx)
_ = config.ListenConfig(vo.ListenParam{
    DataId: "order.yaml", Group: "DEFAULT_GROUP",
    OnChange: func(namespace, group, dataId, data string) {
        fmt.Printf("config changed: %s = %s\n", dataId, data)
    },
})
```

### Lifecycle & Errors

- `Init`: logs and returns immediately when disabled; parses the DSN to build the shared `ClientConfig` and `ServerConfigs`, then creates the two clients in sequence, rolling back the first if the second fails; then fetches the service list to probe liveness (an empty list also counts as success); on `Shutdown`, `CloseClient` closes both in sequence
- Common errors: `ErrNotEnabled`/`ErrNotReady` (plugin disabled/not ready), `ErrInvalidDSN`, `ErrInvalidScheme`

## zookeeper

The ZooKeeper client component for Weaver, based on [go-zookeeper/zk](https://github.com/go-zookeeper/zk). Simply declare `weaver.Ref[zookeeper.T]` to inject a `*zk.Conn`; connection setup and `StateConnected` liveness probing are managed by the framework lifecycle.

### Installation

```bash
go get github.com/jun3372/weaver/plugins/zookeeper

# Anonymous import of the main package completes component registration:
# import _ "github.com/jun3372/weaver/plugins/zookeeper"
```

### Configuration

```yaml
zookeeper:
  Enabled: true
  DSN: "zk://localhost:2181"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the plugin is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `DSN` | `zk://localhost:2181` | Supports multi-host cluster addresses such as `zk://host1:2181,host2:2181` |

Environment variable overrides: `WEAVER_ZOOKEEPER_DSN`, `WEAVER_ZOOKEEPER_ENABLED`.

### Basic Usage

```go
// zk.Conn's Get returns both the data and the Stat
func (l *lockRepo) Get(ctx context.Context, path string) ([]byte, error) {
    conn, err := l.Ref().Client(ctx)
    if err != nil {
        return nil, err
    }
    data, _, err := conn.Get(path)
    return data, err
}
```

### Advanced Scenarios

#### Node Creation and flag Constants

```go
conn, _ := l.Ref().Client(ctx)
// persistent node (flag is 0)
conn.Create("/weaver", []byte{}, 0, zk.WorldACL(zk.PermAll))
// ephemeral node: deleted automatically when the session ends; commonly used for service registration
conn.Create("/weaver/nodes/node-1", []byte("10.0.0.5:8080"),
    zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
// ephemeral sequential node: the classic choice for distributed locks
conn.Create("/weaver/locks/order-", []byte{},
    zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
```

#### Set and Version Control

```go
// Get returns a Stat; Set takes a version for optimistic locking (pass -1 to skip the check)
data, stat, _ := conn.Get("/weaver/config")
_, _ = conn.Set("/weaver/config", []byte("new-value"), stat.Version)
```

#### Existence Watch (ExistsW)

```go
exists, _, ch, _ := conn.ExistsW("/weaver/nodes/node-1")
if exists {
    ev := <-ch
    if ev.Type == zk.EventNodeDeleted {
        fmt.Println("node is gone:", ev.Path)
    }
}
```

### Lifecycle & Errors

- `Init`: logs and returns immediately when disabled; parses the DSN into a host list and calls `zk.Connect` (session timeout 5s, with automatic connection retry internally); if the connection does not reach the `StateConnected` state, it calls `Close` and returns `ErrNotReady`; on `Shutdown`, `Close` closes the connection
- Common errors: `zk.ErrNodeExists` (node already exists), `zk.ErrNoNode` (node does not exist), `ErrNotEnabled`/`ErrNotReady` (plugin disabled/not ready), `ErrInvalidDSN`

## Further Reading

- [Plugins Overview](/en/guide/plugins)
- [Configuration](/en/guide/config)
