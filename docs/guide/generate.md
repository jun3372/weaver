# CLI 工具

Weaver 提供命令行工具 `cmd/weaver`,覆盖项目初始化、组件实现生成与注册代码生成。推荐先安装到本地,后续所有操作都直接使用 `weaver` 命令。

## 安装

```bash
go install github.com/jun3372/weaver/cmd/weaver@latest
```

安装完成后确认可用：

```bash
weaver version
```

不想安装时，也可以用 `go run` 临时执行（下文所有 `weaver` 命令都可以等价替换为 `go run github.com/jun3372/weaver/cmd/weaver`）。

## 初始化项目

`weaver init` 一键生成一个完整可运行的 Weaver 项目:

```bash
weaver init myapp --module example.com/myapp
cd myapp
go run . -conf weaver.yaml
```

生成内容:

- `main.go`:Main 组件(依赖注入示例组件、读取配置)
- `greet/`:示例组件,演示 `weaver.Implements` / `WithConfig` / 生命周期钩子
- `weaver.yaml`:配置文件(app / 组件 / weaver 系统日志)
- 自动执行 `go mod init`、`go get github.com/jun3372/weaver@latest` 与 `weaver generate`(覆盖全部子包)

目录非空时会拒绝执行,加 `--force` 覆盖;`--module` 缺省取目录名。

## 生成组件实现

写好组件接口后,`weaver make` 可以生成实现结构体骨架并自动完成注册:

```bash
weaver make ./greet Echo
```

在接口所在包生成 `echo_impl.go`:

```go
// echo 是 Echo 的组件实现骨架,由 weaver make 生成。
type echo struct {
	weaver.Implements[Echo]
}

func (i *echo) Shout(p0 context.Context, p1 string) (string, error) {
	// TODO: implement
	var (
		r0 string
		r1 error
	)
	return r0, r1
}
```

- 参数缺名时自动补 `p0`、`p1`;返回值以零值变量返回,填充 `// TODO` 即可
- 方法签名必须满足组件方法约定:第一个参数为 `context.Context`,最后一个返回值为 `error`(空接口组件如 `type T interface{}` 也可生成)
- 生成后自动对该包执行 `weaver generate` 完成注册
- 目标文件已存在时拒绝覆盖,加 `--force`;接口名不存在时会列出包内候选

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
