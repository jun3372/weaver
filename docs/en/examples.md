# Examples

Weaver ships several example projects in the repository's `examples` directory, ready to be used as starting points for new projects.

## hello — Hello World

The most basic example, demonstrating component definition, dependency injection and the full lifecycle:

- `main.go`: the main application, referencing the `User` and `Chat` components
- `user/`: the User component implementation
- `chat/`: the Chat component implementation
- `weaver.yaml` / `weaver.toml`: config files in both formats

```bash
cd examples/hello
go run . -conf weaver.yaml
```

## demo — Multi-Component Demo

A more complex example with multiple components, a sub-package component and configuration:

- `main.go`: the main application
- `wechat/`: a component in its own package, demonstrating cross-package references
- `etc/`: config files

```bash
cd examples/demo
go run . -conf etc/weaver.yaml
```

## http — HTTP/TCP/UDP Servers & Config Hot Reload

One process running three servers, demonstrating `weaver.Listener[H]` server components (HTTP/TCP/UDP) and config hot reload:

- `main.go`: main app referencing the three server components
- `server/`: HTTP server whose response changes live with config reloads
- `tcp/`: TCP echo server
- `udp/`: UDP echo server
- `etc/`: config file

```bash
cd examples/http
go run . -conf etc/weaver.yaml
curl localhost:8080                      # hello v1
printf 'ping\n' | nc localhost 8081      # TCP echo: ping
printf 'ping\n' | nc -u localhost 8082   # UDP echo: PING
# edit http.message in etc/weaver.yaml, no restart needed
curl localhost:8080                      # hello v2
```

See [Listener: HTTP / TCP / UDP Server Hosting](/en/guide/listener).

## echo — Listener Examples for All Three Protocols

Four standalone runnable examples of `weaver.Listener[H]` — implement a handler on the component and it is served automatically, no `Start` needed:

- `echo/http`: HTTP Listener + `Mux()` multi-route
- `echo/tcp`: TCP Listener, demonstrating session broadcast with self-exclusion
- `echo/udp`: UDP Listener, demonstrating peer management and targeted send
- `echo/all`: three-protocol gateway in one process (HTTP :8080 / TCP :8081 / UDP :8082)

```bash
cd examples/echo/all
go run . -conf weaver.yaml
curl localhost:8080                      # http ok (tcp/udp also online)
printf 'ping\n' | nc localhost 8081      # TCP echo: ping
printf 'ping\n' | nc -u localhost 8082   # UDP echo: PING
curl localhost:8080/sessions             # current TCP session count
```

## protocol — Private Protocol Parsing

Demonstrates typical patterns for parsing private protocols inside TCP/UDP handlers: SOCKS5 method negotiation (TCP byte streams framed with `io.ReadFull` using a "fixed header + length field") and UDP binary frames (parsed directly with `encoding/binary` at datagram boundaries), with unit tests.

```bash
cd examples/protocol
go test ./...   # protocol parsing tests
go run . -conf weaver.yaml
```

See [Listener: HTTP / TCP / UDP Server Hosting](/en/guide/listener).

## template — Project Template

A minimal project template — the recommended starting point for new projects:

- `main.go`: main application skeleton
- `weaver.yaml` / `weaver.toml`: config file skeletons

## Further Reading

- [Getting Started](/en/guide/getting-started)
- [Core Concepts: Components & Dependency Injection](/en/guide/concepts)
- [Listener: HTTP / TCP / UDP Server Hosting](/en/guide/listener)
