# 快速开始

Weaver 是一个轻量级的 Go 语言应用框架，专注于提供简单、灵活且功能强大的组件化应用程序构建体验。它通过依赖注入、配置管理和生命周期管理等特性，帮助开发者构建模块化、可维护的应用程序。

## 安装

确保你的 Go 版本 >= 1.27，然后安装框架与 CLI 工具：

```bash
# 框架(作为应用依赖)
go get github.com/jun3372/weaver

# CLI 工具(init / generate / make / version)
go install github.com/jun3372/weaver/cmd/weaver@latest
```

## 创建第一个应用

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
        app.Logger(ctx).Info("App running")
        <-ctx.Done()
        return nil
    })
    if err != nil {
        panic(err)
    }
}
```

`weaver.Run` 的第二个参数是主逻辑函数，收到装配完成的 Main 组件实例；如果应用没有额外主逻辑、只需跑组件等退出信号，可用一行简写 `weaver.RunComponent(ctx, (*app)(nil))`（第二个参数仅用于类型推断）。

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

### 3. 添加一个 HTTP 服务

内嵌 `weaver.Listener[weaver.Handler]` 并在 `Init` 中注册路由，框架即自动注入监听地址、启动服务并托管优雅关闭——无需编写 `Start`：

```go
type Api any

type api struct {
    weaver.Implements[Api]
    weaver.Listener[weaver.Handler] `conf:"http"`
}

func (a *api) Init(ctx context.Context) error {
    a.Mux().HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
        _, _ = w.Write([]byte("Hello, Weaver!"))
    })
    return nil
}
```

在 `weaver.yaml` 中补充监听地址：

```yaml
http:
  addr: ":8080"
```

完整代码见 [Listener:HTTP / TCP / UDP 服务托管](/guide/listener)，TCP/UDP 服务、外部框架（gin/echo）接入与安全防护选项也在该页说明。

### 4. 生成组件注册代码

```bash
weaver generate .
```

该命令会在当前包中生成 `weaver_gen.go`，完成组件的自动注册。组件接口变更后必须重新执行；也可用 `//go:generate weaver generate` 挂到 `go generate`。详见[CLI 工具](/guide/generate)。

### 5. 运行应用

```bash
go run main.go -conf weaver.yaml
curl localhost:8080        # Hello, Weaver!
```

## 启动参数

`weaver.Run` 内置了两个命令行参数：

| 参数 | 环境变量 | 说明 |
| --- | --- | --- |
| `-conf` | `SERVICE_CONFIG` | 配置文件路径，未指定时不加载配置 |
| `-version` | `SERVICE_VERSION=true` | 打印版本信息后退出 |

版本信息为对齐的键值表（首行 `Weaver <版本>`，随后 Go / OS/Arch / Commit / Built，无值的行省略），也可通过 `weaver version` 子命令查看：

```bash
$ weaver version
Weaver v0.1.6
  Go:      go1.27.1
  OS/Arch: darwin/arm64
  Commit:  f3dd60d
  Built:   2026-10-03 12:00:00 UTC
```

## CLI 命令一览

| 命令 | 说明 |
| --- | --- |
| `weaver init [dir]` | 初始化一个完整可运行的项目（含 HTTP 服务组件、配置、Makefile、Dockerfile 与编排文件） |
| `weaver generate [packages]` | 生成组件注册代码 `weaver_gen.go` |
| `weaver make <pkgdir> <Iface>` | 为组件接口生成实现结构体骨架 |
| `weaver version` | 输出版本信息 |

## 下一步

- 了解[核心概念：组件与依赖注入](/guide/concepts)
- 阅读[Listener:HTTP / TCP / UDP 服务托管](/guide/listener)
- 阅读[配置管理](/guide/config)与[日志系统](/guide/logger)
- 查看[完整示例](/examples)
