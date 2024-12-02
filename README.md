# Weaver

Weaver 是一个轻量级的 Go 语言应用框架，专注于提供简单、灵活且功能强大的应用程序构建体验。

## 特性

- 🚀 轻量级设计
- 📦 模块化架构
- 🔧 灵活的配置管理
- 🎯 依赖注入支持
- 📝 内置日志系统
- 🛠️ 丰富的工具集

## 安装

确保你的 Go 版本 >= 1.22.7，然后执行以下命令：

```bash
go get github.com/jun3372/weaver
```

## 快速开始

1. 创建一个新的 Go 项目
2. 初始化 Go 模块
3. 添加 Weaver 依赖

### 基础示例

```go
package main

import (
    "context"
    "github.com/jun3372/weaver"
)

type App struct {
    // 你的应用配置
}

func main() {
    ctx := context.Background()
    weaver.Run(ctx, func(ctx context.Context, app *App) error {
        // 你的应用逻辑
        return nil
    })
}
```

## 项目结构

```
.
├── cmd/           # 命令行工具
├── examples/      # 示例代码
│   ├── demo/      # 演示应用
│   ├── hello/     # Hello World 示例
│   └── template/  # 项目模板
├── internal/      # 内部包
├── runtime/       # 运行时支持
├── version/       # 版本信息
├── weaver.go     # 核心包
└── widget.go     # 组件系统
```

## 配置管理

Weaver 使用 [Viper](https://github.com/spf13/viper) 进行配置管理，支持多种配置格式：

- YAML
- JSON
- TOML
- 环境变量
- 命令行参数

### 配置示例

```yaml
# config.yaml
app:
  name: myapp
  port: 8080
```

## 主要功能

### 1. 依赖注入

Weaver 提供了简单而强大的依赖注入机制，帮助你管理应用组件。

### 2. 日志系统

内置 `slog` 支持，提供结构化日志记录功能。

### 3. 信号处理

自动处理系统信号（SIGINT, SIGQUIT, SIGTERM），确保应用优雅退出。

## 示例

查看 `examples` 目录获取更多示例：

- `examples/hello`: 基础示例
- `examples/demo`: 完整应用示例
- `examples/template`: 项目模板

## 依赖

- github.com/pkg/errors
- github.com/spf13/cobra
- github.com/spf13/viper
- golang.org/x/exp
- golang.org/x/tools
- gopkg.in/natefinch/lumberjack.v2

## 版本要求

- Go 1.22.7 或更高版本

## 贡献指南

欢迎提交 Issue 和 Pull Request！

## 许可证

本项目采用 MIT 许可证。
