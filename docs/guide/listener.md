# Listener:自动注入配置与 Handler

`weaver.Listener[H]` 把"内嵌服务组件 + `Start` 中手动 `Serve`"收敛为一个泛型内嵌字段:组件实现 `http.Handler` / `weaver.TCPHandler` / `weaver.UDPPacketHandler` 任意一个并内嵌 Listener,框架即自动注入配置并以组件自身为 handler 启动服务——**无需编写 `Start`**。

```go
type echo struct {
    weaver.Implements[Echo]
    weaver.Listener[weaver.Handler] `conf:"listener"` // 配置自动注入,无需 Start
    weaver.WithConfig[option]       `conf:"listener"` // 可选:业务配置共用同一 key
}

func (i *echo) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
    _, _ = w.Write([]byte("hello"))
}
```

```yaml
listener:
  addr: ":8080"
```

## 类型参数 H 的分派规则

| H 的类型 | 语义 |
|---|---|
| 三种 handler 接口之一(如 `http.Handler`) | 固定绑定该协议 |
| 聚合类型 `weaver.Handler` | 自动探测组件实现的接口:恰好一个则绑定;零个则预留 HTTP(等待 `Handler`/`Mux` 注入);多于一个则装配报错 |
| 其他 | 装配报错 |

组件同时实现多种 handler 接口时,须为每个协议声明具名 Listener、以具体接口作类型参数(见下文[多协议并行](#多协议并行))。

## HTTP handler 的三种来源

优先级从高到低,前三者都未提供时启动报错:

1. **`Handler()` 注册的外部框架引擎**:gin、echo、chi 等一切实现了 `http.Handler` 的类型;
2. **`Mux()` 惰性创建的标准库 `*http.ServeMux`**:多路由便捷方式;
3. **组件自身实现的 `ServeHTTP`**。

```go
// 方式一:Mux() 注册多路由
func (i *api) Init(ctx context.Context) error {
    i.Mux().HandleFunc("GET /hello", i.hello)
    i.Mux().HandleFunc("GET /{$}", i.index) // 未注册路径由 ServeMux 返回 404
    return nil
}

// 方式二:接入 gin 等外部框架
func (i *api) Init(ctx context.Context) error {
    e := gin.New()
    e.GET("/hello", i.hello)
    i.Handler(e)
    return nil
}
```

两种注入都须在 `Init` 中完成;`Mux()` 与 `Handler()` 互斥,后调用者覆盖前者。

## 多协议并行

```go
type gateway struct {
    weaver.Implements[Gateway]
    httpL weaver.Listener[http.Handler]            `conf:"http"`
    tcpL  weaver.Listener[weaver.TCPHandler]       `conf:"tcp"`
    udpL  weaver.Listener[weaver.UDPPacketHandler] `conf:"udp"`
}

func (g *gateway) Init(ctx context.Context) error {
    g.httpL.Mux().HandleFunc("GET /{$}", g.index) // 多协议时按 Listener 分别注入
    return nil
}

func (g *gateway) ServeTCP(ctx context.Context, conn net.Conn) { ... }
func (g *gateway) ServeUDP(ctx context.Context, pkt weaver.UDPPacket) ([]byte, error) { ... }
```

三种服务并行启动、独立配置、独立优雅关闭。

## TCP 会话管理与定点推送

TCP 绑定的 Listener 内置会话表,可在 handler 外主动推送或断开(例如广播、后台任务推送):

```go
for _, s := range i.Sessions() {        // 活跃会话快照(按 ID 排序)
    _ = i.Send(s.ID, []byte("push"))    // 定点发送
    _ = i.CloseConn(s.ID)               // 主动断开
}
```

- `TCPSession` 含 `ID`、`RemoteAddr`、`Conn`、`LastActive`(最近一次成功收/发的时间);
- `Send` 使用连接级写锁,与 handler 内的读写**并发安全**;
- `s.Conn` 即 handler 收到的同一连接,可用指针比较定位自身(广播时排除发送者),但外部写入请一律走 `Send`。

## UDP 对端管理与定点发送

UDP 无连接,框架按"对端地址表"管理:每个发包来源登记为对端,保留最近 5 分钟活跃记录。

```go
for _, p := range i.Peers() {             // 活跃对端快照(按地址排序)
    _ = i.SendTo(p.Addr, []byte("push")) // 定点发送
}
```

## 连接事件回调

组件按需实现以下可选接口即可收到连接事件(框架经类型断言调用,未实现则跳过;回调不应长时间阻塞,其 panic 被框架隔离):

```go
// TCP:连接建立与断开时触发(含 CloseConn 主动断开、活跃超时踢除)
func (i *echo) OnConnect(s weaver.TCPSession)    { ... }
func (i *echo) OnDisconnect(s weaver.TCPSession) { ... }

// UDP:对端首包登记时触发 OnPeerConnect;
// 对端超过 5 分钟未活跃由后台清扫触发 OnPeerDisconnect(不依附于后续收包)
func (i *echo) OnPeerConnect(addr net.Addr)    { ... }
func (i *echo) OnPeerDisconnect(addr net.Addr) { ... }
```

## 活跃超时:踢除静默连接

TCP 连接默认开启**活跃超时**防泄漏:活跃时间由**收发双向的成功操作**刷新——客户端上行数据、服务端推送成功均计入;超过 `activeTimeout` 未发生任何成功收发即判定离线,由后台清扫主动关闭连接(handler 将收到读错误并退出)。

```yaml
tcp:
  addr: ":8081"
  activetimeout: 300s  # 缺省 60s;0 取默认,负值显式关闭
```

配合事件回调可实现"上线/离线"语义的完整闭环:纯接收客户端只要推送持续成功就不会被误踢;完全静默的连接在超时后被回收并触发 `OnDisconnect`。

## 配置与热更新

Listener 的配置项与对应服务组件完全一致(`HTTPOption` / `TCPOption` / `UDPOption`,含[安全防护](/guide/servers#安全配置)各选项),`conf` 标签挂在 Listener 字段上注入;业务配置可内嵌 `WithConfig` 共用同一 key。配置文件变更时自动重新注入,但 `addr` 在启动时读取,修改监听地址仍需重启。

组件内可通过 `Config()` 读取注入的服务配置（热更新后返回最新值），返回值类型与绑定协议对应，经类型断言取用：

```go
opt := i.Config().(weaver.TCPOption) // Addr / ActiveTimeout / MaxConns / ...
i.Logger(ctx).Info("tcp 配置", "addr", opt.Addr, "activeTimeout", opt.ActiveTimeout)
```

## 私有协议解析

TCP handler 收到的是字节流,私有协议需自行分帧;UDP 按 datagram 边界天然成帧。`examples/protocol` 演示了 SOCKS5 方法协商等典型写法:TCP 用 `io.ReadFull` 按"定长头 + 长度域"分帧,UDP 直接 `encoding/binary` 解析二进制帧。

## 示例

| 示例 | 说明 |
|---|---|
| `examples/echo/http` | HTTP Listener + `Mux()` 多路由 |
| `examples/echo/tcp` | TCP Listener + 会话广播 |
| `examples/echo/udp` | UDP Listener + 对端管理 |
| `examples/echo/all` | 三协议网关同跑 |
| `examples/protocol` | SOCKS5 私有协议分帧解析(含单测) |

```bash
cd examples/echo/all
go run . -conf weaver.yaml
```

## 相关阅读

- [服务组件:HTTP / TCP / UDP](/guide/servers)
- [配置管理](/guide/config)
- [生命周期管理](/guide/lifecycle)
