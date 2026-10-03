# 快速开始

Weaver 是一个轻量级的 Go 语言应用框架，专注于提供简单、灵活且功能强大的组件化应用程序构建体验。它通过依赖注入、配置管理和生命周期管理等特性，帮助开发者构建模块化、可维护的应用程序。

## 安装

确保你的 Go 版本 >= 1.27，然后执行以下命令：

```bash
go get github.com/jun3372/weaver
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
# 首次先安装 CLI 工具
go install github.com/jun3372/weaver/cmd/weaver@latest

# 之后直接使用本地 weaver 命令
weaver generate .
```

该命令会在当前包中生成 `weaver_gen.go`，完成组件的自动注册。详见[代码生成工具](/guide/generate)。

### 4. 运行应用

```bash
go run main.go -conf weaver.yaml
```

## 启动参数

`weaver.Run` 内置了两个命令行参数：

| 参数 | 环境变量 | 说明 |
| --- | --- | --- |
| `-conf` | `SERVICE_CONFIG` | 配置文件路径，未指定时不加载配置 |
| `-version` | `SERVICE_VERSION=true` | 打印版本信息后退出 |

## 下一步

- 了解[核心概念：组件与依赖注入](/guide/concepts)
- 阅读[配置管理](/guide/config)与[日志系统](/guide/logger)
- 查看[完整示例](/examples)
