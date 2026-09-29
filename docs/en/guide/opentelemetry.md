# OpenTelemetry Integration

Weaver integrates with [OpenTelemetry](https://opentelemetry.io/) for distributed tracing. Below is a complete example of HTTP service tracing using `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`.

## 1. Configure OpenTelemetry

```yaml
# weaver.yaml
weaver:
  telemetry:
    enabled: true
    service_name: "my-http-service"
    exporter:
      type: "otlp" # supports otlp, jaeger, zipkin
      endpoint: "http://localhost:4318" # OTLP HTTP exporter endpoint
```

## 2. Create an HTTP Server Component

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

	// create a traced HTTP handler
	handler := http.NewServeMux()
	handler.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		// grab the span from the request context
		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(attribute.String("user.id", r.URL.Query().Get("user_id")))

		// business log, automatically carries trace_id / span_id
		s.Logger(r.Context()).Info("Received hello request")

		fmt.Fprintf(w, "Hello, World!")
	})

	// wrap the handler with otelhttp for automatic tracing
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

## 3. Use an HTTP Client in the Application

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

	"myapp/http" // the HTTP server component defined above
)

type app struct {
	weaver.Implements[weaver.Main]
	weaver.WithConfig[options] `conf:"app"`
	httpServer weaver.Ref[http.Server] // reference to the HTTP server component
}

func (a *app) Init(ctx context.Context) error {
	a.Logger(ctx).Info("App initialized")
	return nil
}

// send a traced HTTP request
func (a *app) makeRequest(ctx context.Context, url string) (string, error) {
	// create a traced HTTP client
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// add custom tracing attributes
	ctx, span := otel.Tracer("").Start(ctx, "makeRequest")
	defer span.End()
	span.SetAttributes(attribute.String("request.url", url))

	// business baggage, propagated across services
	b, _ := baggage.New(baggage.Member{Key: "user.id", Value: "12345"})
	ctx = baggage.ContextWithBaggage(ctx, b)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func main() {
	err := weaver.Run(context.Background(), func(ctx context.Context, app *app) error {
		app.Logger(ctx).Info("App running")

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

## Collected Trace Data

With this configuration, the Weaver application automatically collects HTTP tracing data and exports it to the configured OpenTelemetry backend (Jaeger, Zipkin, or an OTLP collector). The data includes:

- Detailed HTTP request and response information
- Request handling time and latency
- Cross-service call relationships
- Custom business attributes
- Errors and exceptions

Component logs automatically carry `trace_id` and `span_id`, so traces can be located directly from logs. See [Logging](/en/guide/logger).
