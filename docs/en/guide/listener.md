# Listener: Automatic Config & Handler Injection

`weaver.Listener[H]` collapses the "embed a server component + manually `Serve` inside `Start`" boilerplate into a single generic embedded field: implement any of `http.Handler` / `weaver.TCPHandler` / `weaver.UDPPacketHandler` on the component, embed a Listener, and the framework injects the configuration and serves the component itself as the handler — **no `Start` required**.

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

All three services start in parallel with independent configuration and independent graceful shutdown.

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

## Configuration and Hot Reload

Listener configuration matches the corresponding server component exactly (`HTTPOption` / `TCPOption` / `UDPOption`, including all [security options](/en/guide/servers#security-options)); the `conf` tag sits on the Listener field, and business config can share the same key via `WithConfig`. Config file changes are re-injected automatically, but `addr` is read at startup — changing the listen address still requires a restart.

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

```bash
cd examples/echo/all
go run . -conf weaver.yaml
```

## Further Reading

- [Server Components: HTTP / TCP / UDP](/en/guide/servers)
- [Configuration](/en/guide/config)
- [Lifecycle Management](/en/guide/lifecycle)
