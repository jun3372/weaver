# Weaver

[![Release](https://img.shields.io/github/v/release/jun3372/weaver)](https://github.com/jun3372/weaver/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver.svg)](https://pkg.go.dev/github.com/jun3372/weaver)
[![Docs](https://img.shields.io/badge/docs-jun3372.github.io%2Fweaver-blue)](https://jun3372.github.io/weaver/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Weaver 是一个基于组件（component）的轻量级 Go 应用框架。通过接口组件、依赖注入与声明式配置，把应用拆解为可独立演进的服务单元；网络服务、生命周期与优雅关闭由框架托管，业务代码只保留逻辑本身。

## 特性

- 📦 **组件化架构**：基于接口的组件系统，`weaver.Ref[T]` 声明依赖，启动时自动装配并支持循环引用
- 🔧 **声明式配置注入**：`weaver.WithConfig[T]` + `conf:` 标签，基于 [Viper](https://github.com/spf13/viper) 支持 YAML / TOML / JSON，**配置热更新无需重启**
- 🌐 **服务组件**：`weaver.HTTPServer` / `TCPServer` / `UDPServer` 声明式托管监听、连接管理与优雅关闭，内置生产级安全防护；`weaver.Listener[H]` 进一步聚合为"实现 handler 即自动 Serve"，支持 TCP 会话 / UDP 对端管理与定点推送
- 🔄 **生命周期管理**：`Init` / `Start` / `Shutdown` 钩子，`Start` 支持长驻阻塞，进程退出时统一优雅关闭
- 📝 **结构化日志**：基于标准库 `slog`，`Logger(ctx)` 自动附加 trace/span ID，支持文件轮转
- 🔍 **OpenTelemetry 兼容**：trace 上下文自动透传到日志，可与 otelhttp 等标准生态无缝集成
- 🛠️ **CLI 工具**：`weaver init` 脚手架、`weaver make` 组件实现生成、`weaver generate` 注册代码生成、`weaver version`

## 环境要求

- Go **1.27** 及以上

## 安装

```bash
go get github.com/jun3372/weaver
```

如需固定到某个发布版本：

```bash
go get github.com/jun3372/weaver@v0.1.3
```

CLI 工具可以独立安装：

```bash
go install github.com/jun3372/weaver/cmd/weaver@latest
```

## 快速开始

### 1. 定义组件

```go
package user

import (
    "context"

    "github.com/jun3372/weaver"
)

type User interface {
    SayHello(ctx context.Context, name string) (string, error)
}

type option struct {
    Greeting string
}

type userImpl struct {
    weaver.Implements[User]
    weaver.WithConfig[option] `conf:"user"`
}

func (u *userImpl) SayHello(ctx context.Context, name string) (string, error) {
    return u.Config().Greeting + ", " + name, nil
}
```

### 2. 编写主应用

```go
package main

import (
    "context"

    "github.com/jun3372/weaver"
    "myapp/user"
)

type options struct {
    AppName string
}

type app struct {
    weaver.Implements[weaver.Main]
    weaver.WithConfig[options] `conf:"app"`
    user weaver.Ref[user.User] // 声明依赖,启动时自动注入
}

func main() {
    err := weaver.Run(context.Background(), func(ctx context.Context, a *app) error {
        greeting, err := a.user.Get().SayHello(ctx, "Weaver")
        if err != nil {
            return err
        }
        a.Logger(ctx).Info("app started", "greeting", greeting)
        <-ctx.Done() // 等待退出信号
        return nil
    })
    if err != nil {
        panic(err)
    }
}
```

### 3. 编写配置文件（weaver.yaml）

```yaml
app:
  appname: myapp

user:
  greeting: "Hello"

weaver:
  logger:
    level: info
    type: json
```

### 4. 生成注册代码并运行

```bash
# 组件接口变更后必须重新生成
weaver generate .

# 运行(-conf 指定配置文件,也可通过 SERVICE_CONFIG 环境变量指定)
go run . -conf weaver.yaml
```

也可以用脚手架一键创建完整项目：

```bash
weaver init myapp && cd myapp && go run . -conf weaver.yaml
```

## 组件系统

组件是实现业务接口的普通结构体，通过内嵌 `weaver.Implements[T]` 声明所属接口。框架在启动时反射实例化全部组件、按 `Ref` 声明装配依赖图，并注入配置与日志：

```go
type app struct {
    weaver.Implements[weaver.Main]

    cache weaver.Ref[cache.Cache]  // 依赖注入(支持循环引用,解析到同一实例)
    user  weaver.Ref[user.User]

    api   weaver.HTTPServer `conf:"api"`   // 服务组件也是组件字段
}
```

- **`weaver.Implements[T]`**：声明组件实现的接口 `T`，自带 `Logger(ctx)`
- **`weaver.Ref[T]`**：声明对其他组件的依赖，`Get()` 获取实例（字段形式声明后由框架自动注入）
- **`weaver.WithConfig[T]`**：声明配置依赖，`Config()` 返回配置副本
- **`weaver.Exec()`**：组件内主动触发应用退出

组件方法调用是进程内的普通接口调用，没有序列化与网络开销。

## 生命周期

组件可按需实现以下钩子（均为可选）：

| 钩子 | 语义 |
|---|---|
| `Init(ctx) error` | 装配完成后同步调用，返回错误将中止启动 |
| `Start(ctx) error` | 并发启动，**支持长驻阻塞**（如 `Serve`）；同步快速失败会中止应用，其余失败或 panic 经 ctx 通知 |
| `Shutdown(ctx) error` | 全部 `Start` 退出后依次调用，用于释放资源 |

进程收到 `SIGINT` / `SIGTERM` / `SIGQUIT` 后：通知所有 `Start` 返回 → 关闭服务组件连接 → 限时等待处理中的请求/连接完成 → 执行组件 `Shutdown`。

## 配置管理

基于 Viper，支持 **YAML / TOML / JSON**。配置通过 `conf:` 标签注入（`weaver:` 与 `config:` 等价）：

```go
type options struct {
    Host string
    Port int
    Auth struct {
        Username string
        Password string
    }
}

type service struct {
    weaver.Implements[Service]
    weaver.WithConfig[options] `conf:"service"` // 从配置文件的 "service" 键加载
}

func (s *service) Init(ctx context.Context) error {
    cfg := s.Config() // 返回配置副本,热更新期间并发安全
    s.Logger(ctx).Info("service config", "host", cfg.Host, "port", cfg.Port)
    return nil
}
```

**配置热更新**：修改配置文件后框架自动重新注入（基于 fsnotify 文件监听），`Config()` 立即返回新值，**服务不中断、无需重启**。注意：

- 修改配置请原地写入（编辑器/`WriteFile` 直接覆盖）；以 rename 方式原子替换文件不会触发热更新
- 服务组件的 `addr` 在 `Serve` 时读取，修改监听地址仍需重启
- 解析失败时保留旧配置并记录错误（fail-open）

系统级配置（日志等）位于 `weaver` 键下：

```yaml
weaver:
  logger:
    level: info       # debug, info, warn, error
    type: json        # json 或 text
    addsource: false  # 日志是否携带源码位置
    component: false  # 日志是否附加组件名
    file:             # 文件输出与轮转(可选)
      filename: "./logs/app.log"
      maxsize: 100    # 单文件最大 MB
      maxage: 7       # 保留天数
      maxbackups: 10
      compress: true
      localtime: true
```

## 服务组件

v0.1.2 起提供三个声明式网络服务组件，监听、accept 循环、连接管理与优雅关闭全部由框架托管；同一进程/组件可声明多个实例。

### HTTP

```go
type api struct {
    weaver.Implements[Api]
    weaver.HTTPServer `conf:"api"`
}

func (i *api) Start(ctx context.Context) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/", handle)
    return i.HTTPServer.Serve(ctx, mux) // 长驻阻塞,ctx 结束后优雅关闭
}
```

### TCP

```go
type echo struct {
    weaver.Implements[Echo]
    weaver.TCPServer `conf:"tcp"`
}

func (i *echo) Start(ctx context.Context) error {
    return i.TCPServer.Serve(ctx, echoHandler{})
}

// conn 的生命周期由框架负责
func (echoHandler) ServeTCP(ctx context.Context, conn net.Conn) {
    // 读写 conn,直到对端关闭
}
```

### UDP

```go
type echo struct {
    weaver.Implements[Echo]
    weaver.UDPServer `conf:"udp"`
}

func (i *echo) Start(ctx context.Context) error {
    return i.UDPServer.Serve(ctx, echoHandler{})
}

// 返回非 nil 字节即作为回包发往来源地址
func (echoHandler) ServeUDP(ctx context.Context, pkt weaver.UDPPacket) ([]byte, error) {
    return pkt.Data, nil
}
```

### Listener：自动注入配置与 Handler

不想写 `Start` 时，改用 `weaver.Listener[H]`：组件实现 `http.Handler` / `TCPHandler` / `UDPPacketHandler` 任意一个并内嵌 Listener，框架即自动注入配置并以组件自身为 handler 启动服务。

```go
type echo struct {
    weaver.Implements[Echo]
    weaver.Listener[weaver.Handler] `conf:"listener"` // 配置自动注入,无需 Start
    weaver.WithConfig[option]       `conf:"listener"` // 可选:业务配置共用同一 key
}

func (i *echo) ServeHTTP(w http.ResponseWriter, _ *http.Request) { ... }
```

HTTP 的 handler 也可以不写在组件上，`Listener` 内置两种注入方式（优先级高于组件自身实现），在 `Init` 中完成即可：

```go
type api struct {
    weaver.Implements[T]
    weaver.Listener[weaver.Handler] `conf:"listener"`
}

// 方式一:Mux() 惰性创建标准库 ServeMux,直接注册多路由
func (i *api) Init(ctx context.Context) error {
    i.Mux().HandleFunc("GET /hello", i.hello)
    i.Mux().HandleFunc("GET /{$}", i.index) // 未注册路径由 ServeMux 返回 404
    return nil
}

// 方式二:Set 接入 gin/echo/chi 等外部框架引擎(一切实现 http.Handler 的类型)
func (i *api) Init(ctx context.Context) error {
    e := gin.New()
    e.GET("/hello", i.hello)
    i.Handler(e)
    return nil
}
```

组件实现多种 handler 接口时，为每个协议声明具名 Listener、以具体接口作类型参数，三种服务并行启动：

```go
type gateway struct {
    weaver.Implements[T]
    httpL weaver.Listener[http.Handler]            `conf:"http"`
    tcpL  weaver.Listener[weaver.TCPHandler]       `conf:"tcp"`
    udpL  weaver.Listener[weaver.UDPPacketHandler] `conf:"udp"`
}

func (g *gateway) Init(ctx context.Context) error {
    g.httpL.Mux().HandleFunc("GET /{$}", g.index) // 多协议时按 Listener 分别注入
    return nil
}
```

TCP/UDP 还内置连接管理与定点发送，可在 handler 外主动推送：

```go
for _, s := range i.Sessions() {          // TCP 活跃会话快照(含 LastActive 最近活跃时间)
    _ = i.Send(s.ID, []byte("push"))      // 定点发送(连接级写锁,与 handler 并发安全)
    _ = i.CloseConn(s.ID)                 // 主动断开
}
for _, p := range i.Peers() {             // UDP 活跃对端快照(保留 5 分钟内活跃)
    _ = i.SendTo(p.Addr, []byte("push")) // 定点发送
}
```

组件按需实现可选接口即可收到连接事件（回调由框架隔离 panic，不应长时间阻塞）：

```go
// TCP:连接建立与断开时触发(含 CloseConn 主动断开、活跃超时踢除)
func (i *echo) OnConnect(s weaver.TCPSession)    { i.Logger(context.Background()).Info("上线", "remote", s.RemoteAddr) }
func (i *echo) OnDisconnect(s weaver.TCPSession) { i.Logger(context.Background()).Info("离线", "id", s.ID) }

// UDP:对端首包登记与超 TTL(5 分钟,后台清扫)离线时触发
func (i *echo) OnPeerConnect(addr net.Addr)    {}
func (i *echo) OnPeerDisconnect(addr net.Addr) {}
```

完整可运行示例见 `examples/echo/{http,tcp,udp,all}` 与 `examples/protocol`（SOCKS5 方法协商等私有协议解析：TCP 字节流用 `io.ReadFull` 按"定长头+长度域"分帧，UDP 按 datagram 边界直接 `encoding/binary` 解析）。

### 内置安全防护

v0.1.3 起默认生效，零配置即获得生产级防护，所有选项可通过配置覆盖（显式负值/`<=0` 关闭）：

| 防护 | 说明 | 默认值 |
|---|---|---|
| handler panic 隔离 | TCP/UDP 处理 goroutine 统一 recover，单连接/单包异常不影响进程 | 始终开启 |
| HTTP 超时 | `readTimeout` / `readHeaderTimeout` / `writeTimeout` / `idleTimeout`，防 slowloris | 30s / 10s / 30s / 120s |
| 请求头上限 | `maxHeaderBytes` | 1MB |
| TCP 连接上限 | `maxConns`，超限立即拒绝 | 不限 |
| TCP 空闲回收 | `connIdleTimeout`，每次收发自动续期 | 不限 |
| TCP 活跃超时 | `activeTimeout`，超过未发生任何成功的收或发（客户端上行、服务端推送成功均计入）即判定离线并踢除，`TCPSession.LastActive` 可查最近活跃时间 | 60s |
| UDP 并发上限 | `maxConcurrent`，超限丢弃报文并告警 | 不限 |

```yaml
tcp:
  addr: ":8081"
  maxConns: 1000
  connIdleTimeout: 300s
  activeTimeout: 300s   # 活跃超时,收发双向成功操作均刷新;缺省 60s,负值关闭
```

完整配置说明见[文档站 · 服务组件](https://jun3372.github.io/weaver/guide/servers)与[文档站 · Listener](https://jun3372.github.io/weaver/guide/listener)。

## 日志

组件通过 `Logger(ctx)` 获取结构化日志器，自动附加 trace/span ID（若 ctx 中存在 OpenTelemetry SpanContext）：

```go
func (s *service) Handle(ctx context.Context) {
    logger := s.Logger(ctx)
    logger.Info("processing request", "requestID", "12345")
    logger.Warn("resource running low", "available", "10%")
    logger.Error("operation failed", "err", err)
}
```

日志格式、级别与文件轮转配置见[配置管理](#配置管理)。

## OpenTelemetry 兼容

Weaver 不绑定特定 tracing 后端，与 OpenTelemetry 生态按标准方式协作：

- **自动透传**：`Logger(ctx)` 从 ctx 提取 `SpanContext`，日志自动携带 `trace.id` / `span.id`，业务代码零改造
- **标准接入**：HTTP 服务可用 [`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) 包装 handler，客户端用 `otelhttp.NewTransport`，导出器按 OTel 官方方式初始化

```go
handler := otelhttp.NewHandler(mux, "server")
return i.HTTPServer.Serve(ctx, handler) // 与服务组件直接组合
```

## 命令行工具

```bash
weaver init [dir]              # 初始化完整可运行项目(main + 示例组件 + 配置 + 自动生成)
weaver make <pkgdir> <Iface>   # 为组件接口生成实现结构体骨架
weaver generate [packages]     # 生成组件注册代码 weaver_gen.go(接口变更后必须重新执行)
weaver version                 # 版本信息(自动回退:ldflags → 模块版本 → VCS 戳记)
```

应用通用标志：`-conf <file>` 指定配置文件（或 `SERVICE_CONFIG` 环境变量）；`-version`（或 `SERVICE_VERSION=true`）打印版本信息。

代码中也可使用 go:generate：

```go
//go:generate weaver generate
```

## 性能

框架在请求热路径上接近零开销（与原生 net/http 对比）：

| 指标 | 数值 |
|---|---|
| HTTP 请求路径开销 | ~4.5%（+1.4µs / +2 allocs per request） |
| 配置读取（热更新安全） | 3.8ns，0 allocs |
| 64 并发压测（M3 实测） | HTTP 95k+ ops/s，TCP 105k+ ops/s，零错误、无 goroutine 泄漏 |

基准与压测可自行复现：

```bash
go test ./examples/http/ -bench . -benchmem
go test ./examples/http/ -stress -run Stress
```

## 项目结构

```
.
├── cmd/weaver/      # CLI:init / make / generate / version
├── examples/
│   ├── hello/       # 最小示例(组件定义与配置)
│   ├── demo/        # 多组件与两种配置注入方式
│   ├── http/        # 单进程 HTTP/TCP/UDP 三服务 + 热更新 + 基准/压测
│   ├── echo/        # Listener 示例:http / tcp / udp 各自独立 + all 三协议同跑
│   ├── protocol/    # 私有协议解析示例(SOCKS5 方法协商 / 二进制帧)
│   └── template/    # 手动注册代码生成的项目模板
├── internal/        # 内部实现(config / generate / reflection / files)
├── runtime/         # codegen 注册表 / logger / version
├── weaver.go        # 公共 API:Run / Implements / Ref / WithConfig
├── listener.go      # Listener 聚合服务组件:自动注入配置与 Handler
├── server.go        # 服务组件:HTTPServer / TCPServer / UDPServer
└── widget.go        # DI 容器:反射实例化、依赖装配、生命周期
```

完整文档（中英双语）见 **[jun3372.github.io/weaver](https://jun3372.github.io/weaver/)**。

## 贡献

欢迎提交 Issue 与 Pull Request。提交前请确保以下命令全部通过：

```bash
go build ./... && go vet ./... && go test ./...
```

涉及组件接口变更时请重新执行 `weaver generate`。

## 许可证

[MIT License](LICENSE)
