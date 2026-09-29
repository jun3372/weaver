# OpenTelemetry 集成

Weaver 支持与 [OpenTelemetry](https://opentelemetry.io/) 集成，实现分布式追踪。以下是使用 `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` 实现 HTTP 服务链路追踪的完整示例。

## 1. 配置 OpenTelemetry

```yaml
# weaver.yaml
weaver:
  telemetry:
    enabled: true
    service_name: "my-http-service"
    exporter:
      type: "otlp" # 支持 otlp, jaeger, zipkin
      endpoint: "http://localhost:4318" # OTLP HTTP Exporter 端点
```

## 2. 创建 HTTP 服务组件

```go
package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jun3372/weaver"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

		// 记录业务日志，自动携带 trace_id / span_id
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

## 3. 在应用中使用 HTTP 客户端

```go
package main

import (
	"context"
	"io"
	"net/http"

	"github.com/jun3372/weaver"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"

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

## 采集到的追踪数据

通过上述配置和代码，Weaver 应用将自动收集 HTTP 服务的链路追踪数据，并发送到配置的 OpenTelemetry 后端（如 Jaeger、Zipkin 或 OTLP 接收器）。追踪数据包括：

- HTTP 请求和响应的详细信息
- 请求处理时间和延迟
- 服务间调用关系
- 自定义添加的业务属性
- 错误和异常信息

同时，组件日志会自动携带 `trace_id` 与 `span_id`，可直接在日志中定位链路。详见[日志系统](/guide/logger)。
