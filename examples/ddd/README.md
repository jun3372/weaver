# ddd — 标准 DDD 分层 HTTP 服务

> 本示例是**独立 Go module**（`go.mod` 通过 replace 指回本地框架与 db 组件），构建/测试请在 `examples/ddd` 目录内执行；仓库根目录的本地 `go.work` 已将其纳入联调。

演示如何用 weaver 组件承载标准 DDD 四层架构：

| 层 | 目录 | 职责 |
|---|---|---|
| 领域层 | `internal/domain` | 实体（User/Order）、值对象（Email/Money/OrderItem）、领域规则与仓储端口。纯 Go，不依赖 weaver/db |
| 应用层 | `internal/application` | 用例编排：注册用户、下单、支付、取消、查询；通过 `weaver.Ref` 依赖领域端口 |
| 基础设施层 | `internal/infrastructure/persistence` | GORM + SQLite 仓储实现、持久化模型与领域模型映射、AutoMigrate |
| 交付层 | `internal/interfaces/http` | HTTP Listener、路由、请求 DTO、统一响应信封 |

## 为什么仓储端口与实现在同一包

weaver 生成器要求 `weaver.Implements[T]` 的接口与实现位于同一包。因此领域层定义纯端口
`domain.UserRepository`，基础设施层声明同包组件接口并内嵌它：

```go
type UserRepository interface {
    domain.UserRepository
}
```

应用层为声明 `weaver.Ref` 需要 import 该接口类型，但业务方法只通过 `domain.UserRepository`
使用它；领域层保持零框架依赖。

## 运行

```bash
cd examples/ddd
go run . -conf etc/weaver.yaml
```

默认使用内存 SQLite，启动即可用。落盘方式见 `etc/weaver.yaml` 注释。

## API

所有响应统一为 `{"code":0,"data":...,"message":"ok"}`；错误时 `code` 非 0。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查 |
| POST | `/api/v1/users` | 注册用户 `{name,email}` |
| GET | `/api/v1/users?offset=0&limit=20` | 用户分页 |
| GET | `/api/v1/users/{id}` | 用户详情 |
| POST | `/api/v1/users/{id}/orders` | 下单 `{items:[{productName,unitAmount,currency,quantity}]}` |
| GET | `/api/v1/users/{id}/orders` | 用户订单分页 |
| GET | `/api/v1/orders/{id}` | 订单详情 |
| POST | `/api/v1/orders/{id}/pay` | 支付（pending → paid） |
| POST | `/api/v1/orders/{id}/cancel` | 取消（pending → cancelled） |

金额单位为分，币种为 3 位字母（如 `CNY`）。

```bash
curl -s -X POST localhost:8080/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"张三","email":"zhangsan@example.com"}'

curl -s -X POST localhost:8080/api/v1/users/1/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"productName":"键盘","unitAmount":39900,"currency":"CNY","quantity":2}]}'

curl -s -X POST localhost:8080/api/v1/orders/1/pay
curl -s -X POST localhost:8080/api/v1/orders/1/cancel   # 40902:已支付不能取消
```

## 测试

```bash
go test ./examples/ddd/...
```

领域规则为表驱动单测；应用层与 HTTP 层用 fake 端口；`main_test.go` 走完整 DI + SQLite 全栈流程。
