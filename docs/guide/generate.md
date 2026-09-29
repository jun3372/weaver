# 代码生成工具

Weaver 提供命令行工具 `cmd/weaver`，用于生成组件注册代码。推荐先安装到本地，后续所有操作都直接使用 `weaver` 命令。

## 安装

```bash
go install github.com/jun3372/weaver/cmd/weaver@latest
```

安装完成后确认可用：

```bash
weaver version
```

不想安装时，也可以用 `go run` 临时执行（下文所有 `weaver` 命令都可以等价替换为 `go run github.com/jun3372/weaver/cmd/weaver`）。

## 生成组件注册代码

```bash
# 在当前包生成
weaver generate .

# 指定包
weaver generate ./user ./chat
```

工具会扫描目标包中嵌入了 `weaver.Implements[T]` 的结构体，为每个包生成 `weaver_gen.go`，其中包含组件接口与实现类型的注册信息。`weaver.Run` 启动时依赖这些注册信息构建组件图。

生成的文件带有 `ignoreWeaverGen` 构建 tag，不会被工具再次扫描，避免重复生成。

## 配合 go:generate

在包中添加 `//go:generate` 注释后，可以直接使用 `go generate`：

```go
//go:generate weaver generate
package main
```

## 查看版本

```bash
# CLI 工具版本
weaver version
```

应用本身也支持 `-version` 参数（或环境变量 `SERVICE_VERSION=true`）打印版本信息：

```bash
go run main.go -version
```
