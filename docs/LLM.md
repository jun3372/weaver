# LLM.md — Weaver 框架智能体开发规范

面向 AI 编码智能体的 Weaver 框架使用规范。目标：让智能体在不读源码的情况下写出正确、可编译、符合惯例的 Weaver 应用。

- 框架版本基准：**v0.1.10**（Go 1.27，可用泛型方法 / `reflect.TypeFor` 等新语言特性）
- 组件库版本基准：`github.com/jun3372/weaver/components/*` v0.1.10

---

## 1. Weaver 是什么

Weaver 是基于组件（component）的轻量级 Go 应用框架，核心能力：

1. **基于接口的组件 + 依赖注入**（`weaver.Ref[T]`）
2. **配置注入与热更新**（`weaver.WithConfig[T]` + `conf:` tag，基于 Viper）
3. **服务托管**（`weaver.Listener[H]`，HTTP / TCP / UDP 自动 Serve 与优雅关闭）
4. **生命周期钩子**（`Init` / `Start` / `Shutdown`）
5. **结构化日志**（`log/slog`，`Logger(ctx)` 自动附加 trace/span ID）
6. **OpenTelemetry trace 上下文透传**
7. **代码生成 CLI**（`weaver generate`）

## 2. 最小可运行应用

```go
package main

import (
	"context"

	"github.com/jun3372/weaver"
)

type app struct {
	weaver.Implements[weaver.Main]
}

func main() {
	if err := weaver.Run(context.Background(), func(ctx context.Context, app *app) error {
		<-ctx.Done()
		return nil
	}); err != nil {
		panic(err)
	}
}
```

**必须运行代码生成**（组件接口变更后同样必须）。先把 CLI 安装到本地，再用本地 `weaver` 执行：

```bash
go install github.com/jun3372/weaver/cmd/weaver@latest
weaver generate <包路径>
```

**不要用 `go run github.com/jun3372/weaver/cmd/weaver` 临时执行**：组件接口变更后统一走"先安装、后生成"的流程，避免组件 module 的 go.sum 被生成器依赖污染，也保证生成器版本与已发布版本一致。

生成结果写入该包的 `weaver_gen.go`（含 `//go:build !ignoreWeaverGen`）。没有它，运行时找不到组件注册信息，直接报错。**绝不手改 `weaver_gen.go`**。

## 3. 组件

### 3.1 定义与注入

组件 = 接口 + 实现结构体。实现结构体嵌入 `weaver.Implements[接口T]`，依赖其他组件用 `weaver.Ref[T]` 字段：

```go
type UserRepo interface {
	Find(ctx context.Context, id int64) (string, error)
}

type userRepo struct {
	weaver.Implements[UserRepo]
	weaver.Ref[db.T]    // 依赖官方 db 组件（接口类型 T 定义在组件包里）
	cache weaver.Ref[redis.T]
}

func (u *userRepo) Find(ctx context.Context, id int64) (string, error) {
	_ = u.Get() // 取被注入的实例（Get() 随字段提升）；具名字段用 u.cache.Get()
	...
}
```

要点：

- `Ref` 字段可以匿名（`u.Get()` 取值）或具名（`u.cache.Get()` 取值），`Get()` 方法随字段提升。
- `weaver.Main` 是特殊组件类型（`type Main any`），每个应用最多一个，作为进程入口。
- 组件间跨包引用即跨包 import 接口包，DI 由框架反射完成，无需手工构造。

### 3.2 生命周期钩子（全部可选）

| 钩子 | 签名 | 语义 |
|---|---|---|
| `Init` | `Init(ctx context.Context) error` | 同步初始化（建连、校验配置），任一失败则启动中止 |
| `Start` | `Start(ctx context.Context) error` | **异步**启动长驻逻辑；同步快速失败会中止进程，其余失败/panic 经 cancel 上报为 ERROR 日志 |
| `Shutdown` | `Shutdown(ctx context.Context) error` | 收到 SIGINT/SIGQUIT/SIGTERM 或 `Exit()` 后调用，做优雅清理 |

`Start` 中必须监听 `ctx.Done()`，否则进程会带着失败的组件继续运行。组件可调用 `Exit()`（由 `Implements` 提供）主动触发优雅退出。

### 3.3 日志

```go
func (u *userRepo) Find(ctx context.Context, id int64) (string, error) {
	u.Logger(ctx).Info("查询用户", "id", id) // 自动携带 trace/span ID
	...
}
```

只用 `slog`；不要引入第三方日志库。日志键值对用 `msg, "key", value` 形式。

## 4. 配置

### 4.1 组件配置注入

```go
type option struct {
	AppName string `default:"demo"` // default tag 提供缺省值
	Port    int    `default:"8080"`
}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.WithConfig[option] `conf:"app"` // conf: 指定配置段名
}
```

- tag 等价写法：`conf:` / `weaver:` / `config:`。
- 读取：`app.Config()` 返回配置**副本**，热更新期间并发安全。
- `default` tag 支持 string / bool / 整数 / float / `time.Duration` / 嵌套 struct；**不支持** slice / map / chan / func。

### 4.2 配置文件与优先级

文件名 `weaver.*` / `config.*`（yaml/yml/toml/json），自动在工作目录与可执行文件目录发现，也可 `-conf <path>` 指定。

**优先级：环境变量 > 配置文件 > default tag。**

环境变量命名：`{PREFIX}_{配置段}_{字段路径}`，默认前缀 `WEAVER`（`SERVICE_ENV_PREFIX` 可改）。例：`app.port` → `WEAVER_APP_PORT`；系统配置 `weaver.logger.level` → `WEAVER_LOGGER_LEVEL`（剥掉 weaver 顶层）。

### 4.3 服务组件配置（Listener）

`Listener[H]` 字段加 `conf:` tag 指定配置段，框架读取对应的 `HTTPOption` / `TCPOption` / `UDPOption`：

```yaml
http:
  Addr: ":8080"
  Prefix: "/api/v1"       # handler 挂载到 /api/v1/ 下
  ShutdownTimeout: 5s
  ReadTimeout: 30s
```

**v0.1.8 起语义变更**：Option 字段零值或负值均表示**关闭**该能力；要框架默认值就别写字段（由 `default` tag 填充）。

## 5. 服务组件（Listener）

服务组件 = 组件内嵌 `weaver.Listener[H]`，框架自动监听、Serve、优雅关闭，**无需自己写 Start**。

```go
type gateway struct {
	weaver.Implements[weaver.Main]
	api weaver.Listener[http.Handler] `conf:"http"`
}

func (g *gateway) Init(ctx context.Context) error {
	// HTTP handler 三种来源（优先级从高到低）：
	g.api.Handler(ginEngine) // 1. 外部框架引擎（gin/echo 等任意 http.Handler）
	g.api.Mux().HandleFunc("/ping", ping) // 2. 惰性创建的标准库 ServeMux
	return nil               // 3. 组件自身实现 ServeHTTP
}
```

- `H` 可为 `http.Handler` / `weaver.TCPHandler`（`ServeTCP(ctx, net.Conn)`）/ `weaver.UDPPacketHandler`（`ServeUDP(ctx, UDPPacket) ([]byte, error)`）或聚合接口 `weaver.Handler`（自动探测实现了的协议）。
- 组件实现多个协议 handler 时，为每个协议声明**具名** Listener 并以具体接口作 `H`。
- **同一组件同一协议只能声明一个 Listener**。
- TCP 连接管理：`Sessions()` / `Send(id, data)` / `CloseConn(id)`；UDP：`Peers()` / `SendTo(addr, data)`。可选实现 `TCPOnConnect` / `TCPOnDisconnect` 等事件回调。
- `Addr()` 返回实际监听地址（端口为 0 时可用于拿随机端口）。

## 6. 官方组件库（components/）

`github.com/jun3372/weaver/components/*` 共 17 个独立 Go module，均为纯 Go 实现（`CGO_ENABLED=0` 可交叉编译）：

| 类别 | 组件 |
|---|---|
| 数据库 | `db`（GORM）、`mongodb` |
| 缓存 | `redis`、`memcached` |
| 搜索 | `elasticsearch`、`meilisearch` |
| 消息队列 | `kafka`、`rabbitmq`、`nats`、`nsq`、`mqtt` |
| 注册/配置中心 | `consul`、`etcd`、`nacos`、`zookeeper` |
| 对象存储 | `s3`（minio-go） |
| 任务调度 | `gocron` |

统一使用模式：

```go
import _ "github.com/jun3372/weaver/components/db" // 匿名导入完成组件注册

type userRepo struct {
	weaver.Implements[UserRepo]
	weaver.Ref[db.T]
}
```

约定：

- **Enabled 开关**：配置默认 `Enabled: false`，未启用时注入调用返回 `ErrNotEnabled`，未就绪返回 `ErrNotReady`，可直接 `errors.Is` 判断降级。不用某组件就不要导入它（不引入其客户端依赖）。
- **配置段**：各组件读取同名配置段（`db:`、`redis:`、`kafka:`…），环境变量如 `WEAVER_DB_ENABLED`。
- **生命周期**：`Init` 建连、`Shutdown` 关闭客户端，对未启用组件安全。
- 组件 module 依赖 `github.com/jun3372/weaver` 的**已发布版本**（当前 v0.1.10）；修改组件时注意其 go.mod 不能指向未发布的版本。

## 7. 常用命令

```bash
go install github.com/jun3372/weaver/cmd/weaver@latest  # 先安装 CLI（generate/init/make 均用本地 weaver，勿用 go run）
go build ./...          # 构建
go vet ./...            # 静态检查
go test ./...           # 测试（检测并发问题用 go test -race）
weaver generate <包路径>  # 代码生成：组件接口变更后必须重新生成
weaver init [dir]       # 项目脚手架
weaver make <pkg> <接口名>  # 为组件接口生成实现结构体
weaver version          # 版本信息
```

- 依赖管理直接编辑 `go.mod` + `go mod tidy`。
- 无 Makefile / CI，验证全靠上述 Go 命令，提交前 build/vet/test 必须全过。

## 8. 编码规范与常见坑

1. **错误处理**：统一用 `github.com/pkg/errors`（`errors.Errorf` / `errors.New`）并附上下文包装；`runtime/codegen` 注册表用标准库 + `%w`，遵循所在文件既有风格。
2. **配置字段是副本**：`Config()` 返回拷贝，长逻辑中应先取出再用，避免热更新导致半新半旧。
3. **不要绕过 DI**：不要在组件里 `new` 其他组件，一律 `Ref[T]`。
4. **组件接口约束**：接口变更后必须重新 `weaver generate`，否则运行时注册缺失。
5. **`-race` 测试**：并发相关改动（配置热更新、组件启动）必须跑 `go test -race`。
6. **端口 0 / 随机端口**：测试中用 `Addr()` 取实际地址。
7. **UDP/TCP Option 语义**：零值即关闭（v0.1.8+），需要默认值依赖 `default` tag 而非手写 0。
8. **测试位置**：根包 `widget_test.go` / `listener_test.go` 覆盖 panic 与热更新竞态；改 `examples/hello` 相关代码务必跑 `examples/hello/main_test.go`。

## 9. 目录结构与文档索引

```
weaver.go / listener.go / server.go / widget.go   框架核心（根包，public API）
cmd/weaver/                                        CLI（init / make / generate / version）
internal/{config,defaults,generate,reflection}     内部实现（外部不可导入）
runtime/{codegen,logger}                           可导出的运行时 API
components/                                        官方组件库（17 个独立 module）
examples/{hello,http,demo,template,protocol}       示例（必须保持可编译）
docs/                                              VitePress 文档站源码
```

- 中文教程：`docs/guide/`（getting-started / concepts / config / listener / logger / lifecycle / generate / opentelemetry / components）
- 英文教程：`docs/en/guide/`
- 组件逐个用法：`components/<name>/README.md` 与 `docs/guide/components/*.md`
- 本仓库贡献规范：`AGENTS.md`
