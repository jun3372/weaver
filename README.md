# Weaver

Weaver 是一个轻量级的 Go 语言应用框架，专注于提供简单、灵活且功能强大的组件化应用程序构建体验。它通过依赖注入、配置管理和生命周期管理等特性，帮助开发者构建模块化、可维护的应用程序。

## 特性

- 🚀 **轻量级设计**：核心代码简洁高效，无过多依赖
- 📦 **组件化架构**：基于接口的组件系统，支持依赖注入
- 🔧 **灵活的配置管理**：支持多种配置格式（YAML、TOML、JSON等）
- 🎯 **依赖注入支持**：通过 `Ref` 和 `WithConfig` 实现组件间依赖和配置注入
- 📝 **内置日志系统**：基于 Go 标准库 `slog` 的结构化日志
- 🔄 **生命周期管理**：组件初始化、启动和关闭的生命周期钩子
- 🛠️ **代码生成工具**：通过 `weaver generate` 自动生成组件注册代码
- 🔍 **OpenTelemetry 集成**：支持分布式追踪

## 安装

确保你的 Go 版本 >= 1.22，然后执行以下命令：

```bash
go get github.com/jun3372/weaver
```

## 快速开始

### 1. 创建主应用

```go
package main

import (
    "context"
    "github.com/jun3372/weaver"
)

type options struct {
    AppName string
    Version string
}

type app struct {
    weaver.Implements[weaver.Main]
    weaver.WithConfig[options] `conf:"app"`
}

func (a *app) Init(ctx context.Context) error {
    a.Logger(ctx).Info("App initialized", "name", a.Config().AppName)
    return nil
}

func main() {
    err := weaver.Run(context.Background(), func(ctx context.Context, app *app) error {
        // 应用逻辑
        app.Logger(ctx).Info("App running")
        <-ctx.Done()
        return nil
    })
    if err != nil {
        panic(err)
    }
}
```

### 2. 创建配置文件 (weaver.yaml)

```yaml
app:
  appname: myapp
  version: 1.0.0

weaver:
  logger:
    level: info
    type: json
    file:
      filename: "./logs/weaver.log"
      maxsize: 100
      maxage: 7
      maxbackups: 10
      compress: true
```

### 3. 生成组件注册代码

```bash
go run github.com/jun3372/weaver/cmd/weaver generate .
```

### 4. 运行应用

```bash
go run main.go -conf weaver.yaml
```

## 组件系统

Weaver 的核心是基于接口的组件系统，它通过依赖注入实现组件间的解耦。

### 定义组件接口

```go
package user

import "context"

type User interface {
    SayHello(ctx context.Context, name string) (string, error)
}
```

### 实现组件

```go
package user

import (
    "context"
    "fmt"
    
    "github.com/jun3372/weaver"
)

type option struct {
    Source string
    Type   string
}

type userImpl struct {
    weaver.Implements[User]
    weaver.WithConfig[option] `conf:"user"`
}

func (u *userImpl) Init(ctx context.Context) error {
    u.Logger(ctx).Info("User component initialized")
    return nil
}

func (u *userImpl) SayHello(ctx context.Context, name string) (string, error) {
    return fmt.Sprintf("Hello, %s!", name), nil
}
```

### 使用组件

```go
type app struct {
    weaver.Implements[weaver.Main]
    weaver.WithConfig[options] `conf:"app"`
    user weaver.Ref[user.User]  // 引用 User 组件
}

func (a *app) Init(ctx context.Context) error {
    // 获取 User 组件实例
    userComponent := a.user.Get()
    
    // 调用组件方法
    greeting, err := userComponent.SayHello(ctx, "World")
    if err != nil {
        return err
    }
    
    a.Logger(ctx).Info(greeting)
    return nil
}
```

## 生命周期钩子

Weaver 组件支持以下生命周期钩子：

- **Init(ctx context.Context) error**：组件初始化时调用
- **Start(ctx context.Context) error**：组件启动时调用，支持长时间运行
- **Shutdown(ctx context.Context) error**：组件关闭时调用

## 配置管理

Weaver 使用 [Viper](https://github.com/spf13/viper) 进行配置管理，支持多种配置格式：

- YAML
- TOML
- JSON
- 环境变量

通过 `WithConfig` 泛型类型和结构体标签，可以将配置自动注入到组件中：

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
    weaver.WithConfig[options] `conf:"service"`  // 从配置中的 "service" 键加载
}

// 访问配置
func (s *service) Init(ctx context.Context) error {
    cfg := s.Config()  // 获取配置
    s.Logger(ctx).Info("Service config", "host", cfg.Host, "port", cfg.Port)
    return nil
}
```

## 日志系统

Weaver 使用 Go 标准库的 `slog` 包提供结构化日志记录：

```go
// 在组件中使用日志
func (a *app) DoSomething(ctx context.Context) {
    logger := a.Logger(ctx)  // 获取带有追踪信息的日志器
    
    logger.Info("Processing request", "requestID", "12345")
    logger.Warn("Resource running low", "resource", "memory", "available", "10%")
    logger.Error("Operation failed", "error", errors.New("connection timeout"))
}
```

日志配置示例：

```yaml
weaver:
  logger:
    level: info       # 日志级别：debug, info, warn, error
    type: json        # 日志格式：json 或 text
    addsource: true   # 是否添加源代码位置
    file:
      filename: "./logs/app.log"  # 日志文件路径
      maxsize: 100               # 单个日志文件最大大小(MB)
      maxage: 7                  # 日志文件保留天数
      maxbackups: 10             # 保留的旧日志文件数量
      compress: true             # 是否压缩旧日志
      localtime: true            # 使用本地时间
```

## OpenTelemetry 集成

Weaver 支持与 OpenTelemetry 集成，实现分布式追踪。以下是使用 `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` 库实现 HTTP 服务链路追踪的示例：

### 1. 配置 OpenTelemetry

```yaml
# weaver.yaml
weaver:
  telemetry:
    enabled: true
    service_name: "my-http-service"
    exporter:
      type: "otlp"      # 支持 otlp, jaeger, zipkin
      endpoint: "http://localhost:4318"  # OTLP HTTP Exporter 端点
```

### 2. 创建 HTTP 服务组件

```go
package http

import (
    "context"
    "fmt"
    "net/http"
    
    "github.com/jun3372/weaver"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

type Server interface {
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
}

type options struct {
    Host string
    Port int
}

type serverImpl struct {
    weaver.Implements[Server]
    weaver.WithConfig[options] `conf:"http"`
    
    server *http.Server
}

func (s *serverImpl) Init(ctx context.Context) error {
    cfg := s.Config()
    addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
    
    // 创建带有追踪的 HTTP 处理器
    handler := http.NewServeMux()
    handler.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
        // 从请求上下文中获取 span
        span := trace.SpanFromContext(r.Context())
        span.SetAttributes(attribute.String("user.id", r.URL.Query().Get("user_id")))
        
        // 记录业务日志，包含追踪信息
        s.Logger(r.Context()).Info("Received hello request")
        
        fmt.Fprintf(w, "Hello, World!")
    })
    
    // 使用 otelhttp 包装 HTTP 处理器，自动添加追踪
    otelHandler := otelhttp.NewHandler(handler, "server",
        otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
    )
    
    s.server = &http.Server{
        Addr:    addr,
        Handler: otelHandler,
    }
    
    s.Logger(ctx).Info("HTTP server initialized", "addr", addr)
    return nil
}

func (s *serverImpl) Start(ctx context.Context) error {
    s.Logger(ctx).Info("Starting HTTP server")
    return s.server.ListenAndServe()
}

func (s *serverImpl) Shutdown(ctx context.Context) error {
    s.Logger(ctx).Info("Shutting down HTTP server")
    return s.server.Shutdown(ctx)
}
```

### 3. 在应用中使用 HTTP 客户端

```go
package main

import (
    "context"
    "io"
    "net/http"
    
    "github.com/jun3372/weaver"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    "go.opentelemetry.io/otel/baggage"
    "go.opentelemetry.io/otel/trace"
    
    "myapp/http" // 引入上面定义的 HTTP 服务组件
)

type app struct {
    weaver.Implements[weaver.Main]
    weaver.WithConfig[options] `conf:"app"`
    httpServer weaver.Ref[http.Server] // 引用 HTTP 服务组件
}

func (a *app) Init(ctx context.Context) error {
    a.Logger(ctx).Info("App initialized")
    return nil
}

// 使用带有追踪的 HTTP 客户端发送请求
func (a *app) makeRequest(ctx context.Context, url string) (string, error) {
    // 创建带有追踪的 HTTP 客户端
    client := &http.Client{
        Transport: otelhttp.NewTransport(http.DefaultTransport),
    }
    
    // 创建请求
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", err
    }
    
    // 添加自定义追踪属性
    ctx, span := otel.Tracer("").Start(ctx, "makeRequest")
    defer span.End()
    span.SetAttributes(attribute.String("request.url", url))
    
    // 添加业务相关的 baggage 信息，它会在服务间传递
    b, _ := baggage.New(baggage.Member{Key: "user.id", Value: "12345"})
    ctx = baggage.ContextWithBaggage(ctx, b)
    
    // 发送请求
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    // 读取响应
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    
    return string(body), nil
}

func main() {
    err := weaver.Run(context.Background(), func(ctx context.Context, app *app) error {
        // 应用逻辑
        app.Logger(ctx).Info("App running")
        
        // 发送带有追踪的 HTTP 请求
        resp, err := app.makeRequest(ctx, "http://localhost:8080/hello?user_id=12345")
        if err != nil {
            app.Logger(ctx).Error("Request failed", "error", err)
        } else {
            app.Logger(ctx).Info("Request succeeded", "response", resp)
        }
        
        <-ctx.Done()
        return nil
    })
    if err != nil {
        panic(err)
    }
}
```

通过上述配置和代码，Weaver 应用将自动收集 HTTP 服务的链路追踪数据，并发送到配置的 OpenTelemetry 后端（如 Jaeger、Zipkin 或 OTLP 接收器）。追踪数据包括：

- HTTP 请求和响应的详细信息
- 请求处理时间和延迟
- 服务间调用关系
- 自定义添加的业务属性
- 错误和异常信息

## 命令行工具

Weaver 提供了命令行工具用于代码生成：

```bash
# 生成组件注册代码
go run github.com/jun3372/weaver/cmd/weaver generate [packages]

# 显示版本信息
go run github.com/jun3372/weaver/cmd/weaver version
```

也可以在代码中使用 `//go:generate` 注释自动生成：

```go
//go:generate weaver generate
package main
```

## 项目结构

```
.
├── cmd/           # 命令行工具
├── examples/      # 示例代码
│   ├── demo/      # 演示应用
│   ├── hello/     # Hello World 示例
│   └── template/  # 项目模板
├── internal/      # 内部包
├── runtime/       # 运行时支持
├── version/       # 版本信息
├── weaver.go      # 核心包
└── widget.go      # 组件系统
```

## 示例

Weaver 提供了多个示例项目，位于 `examples` 目录：

- **hello**：基本的 Hello World 应用，展示了组件定义和使用
- **demo**：更复杂的示例，包含多个组件和配置
- **template**：项目模板，可作为新项目的起点

## 贡献

欢迎贡献代码、报告问题或提出改进建议！

## 许可证

[MIT License](LICENSE)
