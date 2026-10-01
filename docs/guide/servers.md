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
