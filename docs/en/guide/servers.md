# Server Components: HTTP / TCP / UDP

Weaver ships three declarative server components — `weaver.HTTPServer`, `weaver.TCPServer` and `weaver.UDPServer`. Listening, accept loops, connection tracking and graceful shutdown are all handled by the framework; you only declare the address via a `conf:` tag and attach a handler inside `Start`.

## HTTP Server

Embed `weaver.HTTPServer` in your component:

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

Config file:

```yaml
api:
  addr: ":8080"
  shutdownTimeout: 5s
```

`Serve(ctx, handler)` blocks until the context is done (app exit or signal), then shuts down gracefully within `shutdownTimeout` and returns `nil`. Listen failures (e.g. port in use) return an error and abort startup.

## TCP Server

Embed `weaver.TCPServer` and implement `weaver.TCPHandler`:

```go
type echo struct {
    weaver.Implements[Echo]
    weaver.TCPServer `conf:"tcp"`
}

// ServeTCP handles a single connection; its lifecycle is managed by the framework
func (echoHandler) ServeTCP(ctx context.Context, conn net.Conn) {
    // read/write conn until the peer closes
}

func (i *echo) Start(ctx context.Context) error {
    return i.TCPServer.Serve(ctx, echoHandler{})
}
```

When the context is done the framework stops accepting, closes every active connection (unblocking handlers stuck in reads) and waits (bounded) for them to finish.

## UDP Server

Embed `weaver.UDPServer` and implement `weaver.UDPPacketHandler`:

```go
// Returning non-nil bytes sends a reply back to the source address; return nil for no reply
func (echoHandler) ServeUDP(ctx context.Context, pkt weaver.UDPPacket) ([]byte, error) {
    return pkt.Data, nil
}

func (i *echo) Start(ctx context.Context) error {
    return i.UDPServer.Serve(ctx, echoHandler{})
}
```

The read loop, concurrent dispatch and reply writes are handled by the framework.

## Security Options

Since v0.1.3 the server components ship with production-grade protections **enabled by default with zero configuration**. Every option can be tuned via its `conf` section; setting a negative/`<=0` value disables the corresponding limit.

### Handler Panic Isolation

The goroutines serving TCP connections and UDP packets are recovered by the framework: a panicking handler is logged as ERROR (with the peer address) and its connection is closed / packet dropped — **the process keeps running**.

### HTTP Timeouts and Header Limits (anti-slowloris)

```yaml
api:
  addr: ":8080"
  readTimeout: 30s         # full request read timeout, default 30s
  readHeaderTimeout: 10s   # request header read timeout, default 10s, primary slowloris defense
  writeTimeout: 30s        # response write timeout, default 30s; set to -1s for long-lived streaming responses (SSE etc.)
  idleTimeout: 120s        # keep-alive idle timeout, default 120s
  maxHeaderBytes: 1048576  # max request header size, default 1MB
```

### TCP Connection Limit and Idle Timeout

```yaml
tcp:
  addr: ":8081"
  maxConns: 1000           # max concurrent connections, excess are rejected immediately; <=0 unlimited (default)
  connIdleTimeout: 300s    # idle timeout, renewed on every read/write so active connections are unaffected; <=0 unlimited (default)
  activeTimeout: 60s       # active timeout, refreshed by successful reads AND writes; expired connections are reaped and trigger OnDisconnect; default 60s, negative disables
```

### UDP Concurrency Limit

```yaml
udp:
  addr: ":8082"
  maxConcurrent: 512       # max in-flight packet handlers, excess packets are dropped with a warning; <=0 unlimited (default)
```

## Multiple Servers in One Process

Declare multiple named instances in a single component (each with its own `conf` key), or spread them across components:

```go
type gateway struct {
    weaver.Implements[Gateway]
    api   weaver.HTTPServer `conf:"api"`   // :8080
    admin weaver.HTTPServer `conf:"admin"` // :8081
}
```

## Config Hot Reload

Server components share the same injection pipeline as `WithConfig`; config file changes are re-injected automatically (see [Configuration](/en/guide/config)). Semantic notes:

- **`Addr` is read when `Serve` starts**: changing the listen address requires a process restart;
- Business values read inside your handler (e.g. a greeting message) **take effect on every request**, no restart needed.

See `examples/http` for a complete demo: one process running HTTP (:8080), TCP (:8081) and UDP (:8082) with config hot reload.

```bash
cd examples/http
go run . -conf etc/weaver.yaml
curl localhost:8080        # hello v1
# edit http.message in etc/weaver.yaml, no restart needed
curl localhost:8080        # hello v2
printf 'ping\n' | nc localhost 8081      # TCP echo
printf 'ping\n' | nc -u localhost 8082   # UDP echo
```

## Further Reading

- [Listener: Automatic Config & Handler Injection](/en/guide/listener)
- [Configuration](/en/guide/config)
- [Lifecycle Management](/en/guide/lifecycle)
