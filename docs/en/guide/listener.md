# Listener: HTTP / TCP / UDP Server Hosting

`weaver.Listener[H]` collapses the declaration, configuration injection and lifecycle management of network services into a single generic embedded field: implement any of `http.Handler` / `weaver.TCPHandler` / `weaver.UDPPacketHandler` on the component, embed a Listener, and the framework injects the configuration, serves the service and manages graceful shutdown — **no `Start` required**.

```go
type echo struct {
    weaver.Implements[Echo]
    weaver.Listener[weaver.Handler] `conf:"listener"` // config injected automatically, no Start
    weaver.WithConfig[option]       `conf:"listener"` // optional: share the same config key
}

func (i *echo) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
    _, _ = w.Write([]byte("hello"))
}
```

```yaml
listener:
  addr: ":8080"
```

## Type Parameter Dispatch Rules

| Type of H | Semantics |
|---|---|
| One of the three handler interfaces (e.g. `http.Handler`) | Binds to that protocol |
| Aggregate type `weaver.Handler` | Auto-detects the interfaces the component implements: exactly one binds; zero reserves HTTP (awaiting `Handler`/`Mux` injection); more than one is an assembly error |
| Anything else | Assembly error |

When a component implements multiple handler interfaces, declare a named Listener per protocol with the specific interface as the type parameter (see [Multiple Protocols](#multiple-protocols) below).

## Three Sources for the HTTP Handler

Priority from high to low; startup fails if none is available:

1. **An external framework engine registered via `Handler()`**: gin, echo, chi — anything implementing `http.Handler`;
2. **A lazily created standard `*http.ServeMux` via `Mux()`**: a convenience for multiple routes;
3. **The component's own `ServeHTTP`**.

```go
// Option 1: register multiple routes via Mux()
func (i *api) Init(ctx context.Context) error {
    i.Mux().HandleFunc("GET /hello", i.hello)
    i.Mux().HandleFunc("GET /{$}", i.index) // unregistered paths return 404 from ServeMux
    return nil
}

// Option 2: integrate gin or another framework
func (i *api) Init(ctx context.Context) error {
    e := gin.New()
    e.GET("/hello", i.hello)
    i.Handler(e)
    return nil
}
```

Both injections must happen inside `Init`; `Mux()` and `Handler()` are mutually exclusive — the later call overrides the former.

## Multiple Protocols

```go
type gateway struct {
    weaver.Implements[Gateway]
    httpL weaver.Listener[http.Handler]            `conf:"http"`
    tcpL  weaver.Listener[weaver.TCPHandler]       `conf:"tcp"`
    udpL  weaver.Listener[weaver.UDPPacketHandler] `conf:"udp"`
}

func (g *gateway) Init(ctx context.Context) error {
    g.httpL.Mux().HandleFunc("GET /{$}", g.index) // inject per Listener for multi-protocol
    return nil
}

func (g *gateway) ServeTCP(ctx context.Context, conn net.Conn) { ... }
func (g *gateway) ServeUDP(ctx context.Context, pkt weaver.UDPPacket) ([]byte, error) { ... }
```

All three services start in parallel with independent configuration and independent graceful shutdown. Note: **each protocol allows only one Listener per component** (validated at assembly time) — for multiple instances of the same protocol, split them across components.

## Graceful Shutdown

When the context is done (app exit or signal), the shutdown semantics per protocol:

- **HTTP**: stops accepting new requests and waits for in-flight ones within `shutdownTimeout`; listen failures (e.g. port in use) abort startup with an error;
- **TCP**: stops accepting, closes every active connection (unblocking handlers stuck in reads) and waits (bounded) for them to finish;
- **UDP**: closes the read loop and waits (bounded) for in-flight packets.

## TCP Session Management and Targeted Push

A TCP-bound Listener keeps a session table so you can push to or disconnect clients outside the handler (broadcasts, background jobs, etc.):

```go
for _, s := range i.Sessions() {        // snapshot of active sessions (sorted by ID)
    _ = i.Send(s.ID, []byte("push"))    // targeted send
    _ = i.CloseConn(s.ID)               // proactive disconnect
}
```

- `TCPSession` carries `ID`, `RemoteAddr`, `Conn` and `LastActive` (the time of the last successful read *or* write);
- `Send` uses a per-connection write lock and is **safe to call concurrently** with the handler;
- `s.Conn` is the same connection the handler received — compare pointers to identify yourself (e.g. exclude the sender when broadcasting), but always write externally via `Send`.

## UDP Peer Management and Targeted Send

UDP is connectionless; the framework tracks peers by address: every packet source is registered as a peer, keeping the last 5 minutes of activity.

```go
for _, p := range i.Peers() {            // snapshot of active peers (sorted by address)
    _ = i.SendTo(p.Addr, []byte("push")) // targeted send
}
```

## Connection Event Callbacks

Implement any of the following optional interfaces to receive connection events (invoked via type assertion; skipped when not implemented; callbacks must not block for long and their panics are isolated by the framework):

```go
// TCP: fired on connect and disconnect (including CloseConn and active-timeout kicks)
func (i *echo) OnConnect(s weaver.TCPSession)    { ... }
func (i *echo) OnDisconnect(s weaver.TCPSession) { ... }

// UDP: OnPeerConnect fires when a peer is first registered;
// OnPeerDisconnect fires from the background sweeper after 5 minutes of inactivity
// (not tied to any subsequent packet)
func (i *echo) OnPeerConnect(addr net.Addr)    { ... }
func (i *echo) OnPeerDisconnect(addr net.Addr) { ... }
```

## Active Timeout: Kicking Silent Connections

TCP connections enable an **active timeout** by default to prevent leaks: the activity clock is refreshed by **successful operations in both directions** — client upstream data and successful server pushes both count. When no successful read or write happens within `activeTimeout`, the connection is deemed offline and proactively closed by the background sweeper (the handler receives a read error and exits).

```yaml
tcp:
  addr: ":8081"
  activetimeout: 300s  # default 60s; 0 = default, negative disables
```

Combined with the event callbacks this closes the online/offline loop: receive-only clients survive as long as pushes keep succeeding, while fully silent connections are reaped on timeout and trigger `OnDisconnect`.

## Security Options

Since v0.1.3 the servers ship with production-grade protections **enabled by default with zero configuration**. Every option can be tuned via its `conf` section; setting a negative/`<=0` value disables the corresponding limit.

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

## Configuration and Hot Reload

Listener configuration uses `HTTPOption` / `TCPOption` / `UDPOption` (including all [security options](#security-options)); the `conf` tag sits on the Listener field, and business config can share the same key via `WithConfig`. Config file changes are re-injected automatically, but `addr` is read at startup — changing the listen address still requires a restart.

Inside the component, `Config()` reads the injected server config (returning the latest value after hot reload). The returned type matches the bound protocol; type-assert to use it:

```go
opt := i.Config().(weaver.TCPOption) // Addr / ActiveTimeout / MaxConns / ...
i.Logger(ctx).Info("tcp config", "addr", opt.Addr, "activeTimeout", opt.ActiveTimeout)
```

## Private Protocol Parsing

TCP handlers receive a byte stream, so private protocols need their own framing; UDP frames naturally at datagram boundaries. `examples/protocol` demonstrates typical patterns such as the SOCKS5 method negotiation: TCP frames with `io.ReadFull` using a "fixed header + length field", UDP parses binary frames directly with `encoding/binary`.

## Examples

| Example | Description |
|---|---|
| `examples/echo/http` | HTTP Listener + `Mux()` multi-route |
| `examples/echo/tcp` | TCP Listener + session broadcast |
| `examples/echo/udp` | UDP Listener + peer management |
| `examples/echo/all` | Three-protocol gateway in one process |
| `examples/protocol` | SOCKS5 private protocol framing (with unit tests) |
| `examples/http` | HTTP/TCP/UDP servers + config hot reload |

```bash
cd examples/echo/all
go run . -conf weaver.yaml
```

`examples/http` demonstrates config hot reload: after editing `http.message` in `etc/weaver.yaml`, the HTTP response changes immediately without a restart:

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

- [Configuration](/en/guide/config)
- [Lifecycle Management](/en/guide/lifecycle)
