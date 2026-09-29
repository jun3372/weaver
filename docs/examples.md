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

## template — 项目模板

最小化的项目模板，建议新项目从复制 `examples/template` 开始：

- `main.go`：主应用骨架
- `weaver.yaml` / `weaver.toml`：配置文件骨架

## 相关阅读

- [快速开始](/guide/getting-started)
- [核心概念：组件与依赖注入](/guide/concepts)
