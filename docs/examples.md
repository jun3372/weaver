# 示例

Weaver 提供了多个示例项目，位于仓库的 `examples` 目录，可以直接作为新项目的起点。

## hello — Hello World

最基本的示例，展示了组件的定义、依赖注入和完整生命周期：

- `main.go`：主应用，引用 `User` 与 `Chat` 组件
- `user/`：User 组件实现
- `chat/`：Chat 组件实现
- `weaver.yaml` / `weaver.toml`：两种格式的配置文件示例

```bash
cd examples/hello
go run . -conf weaver.yaml
```

## demo — 多组件演示

更复杂的示例，包含多个组件、子包组件和配置：

- `main.go`：主应用
- `wechat/`：独立包中的组件，演示跨包组件引用
- `etc/`：配置文件

```bash
cd examples/demo
go run . -conf etc/weaver.yaml
```

## http — HTTP/TCP/UDP 服务与配置热更新

单进程同时运行三个服务，演示 `weaver.Listener[H]` 服务组件（HTTP/TCP/UDP 三协议）与配置热更新：

- `main.go`：主应用，引用三个服务组件
- `server/`：HTTP 服务，响应内容随配置热更新实时变化
- `tcp/`：TCP echo 服务
- `udp/`：UDP echo 服务
- `etc/`：配置文件

```bash
cd examples/http
go run . -conf etc/weaver.yaml
curl localhost:8080                      # hello v1
printf 'ping\n' | nc localhost 8081      # TCP echo: ping
printf 'ping\n' | nc -u localhost 8082   # UDP echo: PING
# 修改 etc/weaver.yaml 中 http.message 后无需重启
curl localhost:8080                      # hello v2
```

详见 [Listener:HTTP / TCP / UDP 服务托管](/guide/listener)。

## echo — Listener 三协议示例

`weaver.Listener[H]` 的四个独立可运行示例，组件实现 handler 即自动 Serve，无需 `Start`：

- `echo/http`：HTTP Listener + `Mux()` 多路由
- `echo/tcp`：TCP Listener，演示会话广播与排除自身
- `echo/udp`：UDP Listener，演示对端管理与定点发送
- `echo/all`：三协议网关同跑（HTTP :8080 / TCP :8081 / UDP :8082）

```bash
cd examples/echo/all
go run . -conf weaver.yaml
curl localhost:8080                      # http ok, tcp/udp 也在线
printf 'ping\n' | nc localhost 8081      # TCP echo: ping
printf 'ping\n' | nc -u localhost 8082   # UDP echo: PING
curl localhost:8080/sessions             # 当前 TCP 会话数
```

## protocol — 私有协议解析

演示在 TCP/UDP handler 中解析私有协议的典型写法：SOCKS5 方法协商（TCP 字节流用 `io.ReadFull` 按"定长头 + 长度域"分帧）与 UDP 二进制帧（按 datagram 边界直接 `encoding/binary` 解析），含单元测试。

```bash
cd examples/protocol
go test ./...   # 协议解析单测
go run . -conf weaver.yaml
```

详见 [Listener:HTTP / TCP / UDP 服务托管](/guide/listener)。

## template — 项目模板

最小化的项目模板，建议新项目从复制 `examples/template` 开始：

- `main.go`：主应用骨架
- `weaver.yaml` / `weaver.toml`：配置文件骨架

## 相关阅读

- [快速开始](/guide/getting-started)
- [核心概念：组件与依赖注入](/guide/concepts)
- [Listener:HTTP / TCP / UDP 服务托管](/guide/listener)
