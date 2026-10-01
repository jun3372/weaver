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

One process running three servers, demonstrating the server components (`weaver.HTTPServer` / `TCPServer` / `UDPServer`) and config hot reload:

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

See [Server Components](/en/guide/servers).

## template — Project Template

A minimal project template — the recommended starting point for new projects:

- `main.go`: main application skeleton
- `weaver.yaml` / `weaver.toml`: config file skeletons

## Further Reading

- [Getting Started](/en/guide/getting-started)
- [Core Concepts: Components & Dependency Injection](/en/guide/concepts)
