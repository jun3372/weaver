# AGENTS.md

面向 AI 编码代理的项目指南。请先阅读本文件再动手改代码。

## 项目概述

`github.com/jun3372/weaver` 是一个基于组件（component）的轻量级 Go 应用框架（Go 1.27，见 go.mod；生成代码使用 `reflect.TypeFor`，规范以 1.27 语言特性为准，如泛型方法），文档与注释以中文为主。源自 `github.com/ServiceWeaver/weaver` 的重写/衍生版本，**不要假设 ServiceWeaver 的 API 在此可用**——部分文件（如 `runtime/codegen/listeners.go`、`runtime/codegen/graph.go`、`internal/files`）仍保留 Google LLC Apache-2.0 头部，属于遗留代码，扩展前先确认是否仍在使用。

核心能力：
- 基于接口的组件 + 依赖注入（`weaver.Ref[T]`）
- 配置注入（`weaver.WithConfig[T]` + `conf:` tag），基于 Viper 支持热更新
- 生命周期钩子（`Init` / `Start` / `Shutdown`），Start 异步启动（失败经 cancel 上报）
- 基于 `log/slog` 的日志（`weaver.Implements[T]` 自带 `Logger(ctx)`，自动注入 trace/span ID）
- OpenTelemetry trace 上下文透传
- `weaver generate` 代码生成 CLI（cobra 实现）

## 常用命令

```bash
# 构建 / 静态检查 / 测试（提交前必须全部通过）
go build ./...
go vet ./...
go test ./...

# 代码生成：组件接口变更后必须重新生成
go run github.com/jun3372/weaver/cmd/weaver generate <包路径>
```

- 无 Makefile、无 CI 配置，验证全靠上述 Go 命令。
- 测试位置：`widget_test.go`（根包，覆盖组件 panic 与配置热更新竞态，须用 `go test -race` 才能暴露竞争）、`examples/hello/main_test.go`、`runtime/logger/logger_test.go`。改完 `examples/hello` 相关代码务必跑它，否则测试不会暴露问题。
- `weaver generate` 会写入 `weaver_gen.go`（含 `//go:build !ignoreWeaverGen`），生成失败时优先检查组件接口是否满足代码生成器要求。

## 目录结构

| 路径 | 职责 |
|---|---|
| `weaver.go` | 公共 API：`Main`、`Run[T,P]`、`Implements[T]`、`WithConfig[T]`、`Ref[T]`、`Exec` |
| `widget.go` | DI 容器（unexported `widget`）：反射实例化组件、注入 Ref/Config/Logger、errgroup 启动、配置变更重启与优雅关闭 |
| `version/` | 版本信息，`-version` 标志或 `SERVICE_VERSION=true` 环境变量输出 |
| `cmd/weaver/` | cobra CLI：`generate`、`version`（`init` 是空 stub） |
| `internal/config` | 系统配置结构体，配置 tag 为 `["weaver", "config", "conf"]` |
| `internal/generate` | 代码生成引擎 |
| `internal/reflection` | `Type[T]`、`ComponentName[T]` 等反射工具 |
| `internal/files` | 原子文件写入（遗留代码） |
| `runtime/codegen` | 组件注册表：`Register` / `Registered` / `Find`，由生成的 `init()` 填充 |
| `runtime/logger` | slog 封装（`New(opts...)`），含 trace/span ID 常量 |
| `runtime/version` | ServiceWeaver API 版本常量，基本为遗留代码 |
| `examples/hello` | 最小示例，含 `main_test.go`，配置文件 `weaver.yaml` / `weaver.toml` |
| `examples/demo` | wechat 组件示例，演示 `Config()` 与具名 `WithConfig` 字段两种注入方式 |
| `examples/template` | 在 `init()` 中手动注册代码生成结果（无 `weaver_gen.go`）的写法 |

依赖方向：根包 → `runtime/codegen` + `runtime/logger` + `internal/*`；生成的 `weaver_gen.go` → `runtime/codegen.Register`。

## 编码约定

- **错误处理**：统一用 `github.com/pkg/errors`（`errors.Errorf` / `errors.New`）并附带上下文包装；`runtime/codegen` 注册表用标准库 + `%w`，保持所在文件既有风格。
- **日志**：业务/组件代码用 `Logger(ctx)`；不要引入第三方日志库。示例与内部代码中存在中文日志消息，新增日志可沿用中文。
- **注释**：中英混用，新增注释建议中文，与 README 一致。
- **提交信息**：遵循 conventional commits（`feat:`、`fix:`、`refactor:`、`docs:`、`chore:`），参考 `git log` 既有风格。
- **配置 tag**：组件配置字段使用 `conf:` tag（等价于 `weaver:` / `config:`）。

## 代理注意事项（重要）

1. **绝不手改 `weaver_gen.go`**——它是生成文件，组件接口变更后重新运行 `weaver generate`。
2. **`internal/` 不可被外部导入**；外部使用方只能依赖根包与 `runtime/` 导出 API。
3. **`widget.go` 使用 `unsafe.Pointer` 向未导出字段注入依赖**——重构时保持注入路径不变，勿"顺手清理"。
4. `examples/` 下的三个示例必须保持可编译；`examples/hello/main_test.go` 依赖其中的 `run()` 函数。
5. 已知的无害瑕疵，不必修复除非被要求：cobra 命令 `Short` 描述仍写着 "Hugo"（复制粘贴残留）；`cmd/weaver/init` 是空 stub；`runtime/version` 基本未被使用。
6. 依赖管理通过 `go.mod` 直接编辑 + `go mod tidy`，无额外工具。
