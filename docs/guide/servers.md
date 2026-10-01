# 服务组件：HTTP / TCP / UDP

Weaver 提供三个声明式服务组件 `weaver.HTTPServer`、`weaver.TCPServer`、`weaver.UDPServer`：监听、accept 循环、连接管理与优雅关闭全部由框架托管，用户只需通过 `conf:` 标签声明端口配置，并在 `Start` 中挂载 handler。

## HTTP 服务

在组件中内嵌 `weaver.HTTPServer` 并打上 `conf` 标签：

```go
type api struct {
    weaver.Implements[Api]
    weaver.HTTPServer `conf:"api"`
}

func (i *api) Start(ctx context.Context) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/", handle)
    return i.HTTPServer.Serve(ctx, mux)
}
```

对应的配置文件：

```yaml
api:
  addr: ":8080"
  shutdownTimeout: 5s
```

`Serve(ctx, handler)` 会长驻阻塞，直到 ctx 结束（应用退出或收到信号），随后按 `shutdownTimeout` 优雅关闭并返回 `nil`。监听失败（如端口占用）会返回错误并中止应用启动。

## TCP 服务

内嵌 `weaver.TCPServer`，实现 `weaver.TCPHandler` 接口：

```go
type echo struct {
    weaver.Implements[Echo]
    weaver.TCPServer `conf:"tcp"`
}

// ServeTCP 处理单条连接;conn 的生命周期由框架负责
func (echoHandler) ServeTCP(ctx context.Context, conn net.Conn) {
    // 读写 conn,直到对端关闭
}

func (i *echo) Start(ctx context.Context) error {
    return i.TCPServer.Serve(ctx, echoHandler{})
}
```

ctx 结束后，框架会停止 accept、逐个关闭活跃连接（解除 handler 的读阻塞）并限时等待全部退出。

## UDP 服务

内嵌 `weaver.UDPServer`，实现 `weaver.UDPPacketHandler` 接口：

```go
// 返回非 nil 字节即作为回包发往来源地址;返回 nil 表示不回复
func (echoHandler) ServeUDP(ctx context.Context, pkt weaver.UDPPacket) ([]byte, error) {
    return pkt.Data, nil
}

func (i *echo) Start(ctx context.Context) error {
    return i.UDPServer.Serve(ctx, echoHandler{})
}
```

框架负责读循环、并发派发与回包写回。

## 安全配置

v0.1.3 起服务组件内置生产级防护，**零配置即默认生效**；所有选项均可通过 `conf` 段调整，显式设为负值/`<=0` 可关闭对应限制。

### handler panic 隔离

TCP 连接与 UDP 报文的处理 goroutine 由框架统一 recover：业务 handler panic 只记录 ERROR 日志（含对端地址）并关闭该连接/丢弃该包，**不会击穿进程**，服务继续运行。

### HTTP 超时与请求头上限（防 slowloris 慢速攻击）

```yaml
api:
  addr: ":8080"
  readTimeout: 30s         # 整个请求读取超时,缺省 30s
  readHeaderTimeout: 10s   # 请求头读取超时,缺省 10s,slowloris 主防线
  writeTimeout: 30s        # 响应写出超时,缺省 30s;长连接流式响应(SSE 等)需显式设为 -1s 关闭
  idleTimeout: 120s        # keep-alive 空闲超时,缺省 120s
  maxHeaderBytes: 1048576  # 请求头大小上限,缺省 1MB
```

### TCP 连接数与空闲超时

```yaml
tcp:
  addr: ":8081"
  maxConns: 1000           # 最大并发连接数,超过立即拒绝;<=0 不限(缺省)
  connIdleTimeout: 300s    # 空闲超时,每次收发自动续期,活跃连接不受影响;<=0 不限(缺省)
```

### UDP 报文处理并发上限

```yaml
udp:
  addr: ":8082"
  maxConcurrent: 512       # 单包处理最大并发数,超限丢弃报文并告警;<=0 不限(缺省)
```

## 同一进程运行多个服务

服务组件支持在一个组件中声明多个具名实例（各自独立的 `conf` key），也可以分布在多个组件中：

```go
type gateway struct {
    weaver.Implements[Gateway]
    api   weaver.HTTPServer `conf:"api"`   // :8080
    admin weaver.HTTPServer `conf:"admin"` // :8081
}
```

## 配置热更新

服务组件的配置与 `WithConfig` 走同一注入链路，配置文件变更时自动重新注入（详见[配置管理](/guide/config)）。注意语义区别：

- **`Addr` 在 `Serve` 时读取**：修改监听地址需要重启进程才能生效；
- handler 内部读取的业务配置（如响应文案）**每次请求实时生效**，无需重启。

完整示例见 `examples/http`：单进程同时运行 HTTP（:8080）、TCP（:8081）、UDP（:8082）三个服务，并演示配置热更新。

```bash
cd examples/http
go run . -conf etc/weaver.yaml
curl localhost:8080        # hello v1
# 修改 etc/weaver.yaml 中 http.message 后无需重启
curl localhost:8080        # hello v2
printf 'ping\n' | nc localhost 8081      # TCP echo
printf 'ping\n' | nc -u localhost 8082   # UDP echo
```

## 相关阅读

- [配置管理](/guide/config)
- [生命周期管理](/guide/lifecycle)
