# 生命周期管理

Weaver 组件支持以下生命周期钩子，全部为可选实现：

| 钩子 | 签名 | 说明 |
| --- | --- | --- |
| Init | `Init(ctx context.Context) error` | 组件初始化时调用，返回 error 会阻断启动 |
| Start | `Start(ctx context.Context) error` | 组件启动时调用，适合长时间运行的服务 |
| Shutdown | `Shutdown(ctx context.Context) error` | 组件关闭时调用，用于释放资源 |

## Init

`Init` 在组件实例化完成后立即调用，此时依赖注入（`WithConfig`、`Ref`）已经完成，可以安全地访问配置和依赖的组件。

## Start

实现了 `Start` 的组件会在 `weaver.Run` 启动主逻辑前**并发**启动。适合长驻的后台任务：

```go
func (s *worker) Start(ctx context.Context) error {
    t := time.NewTicker(time.Second)
    defer t.Stop()
    for {
        select {
        case <-ctx.Done():
            return nil // ctx 结束,任务退出
        case <-t.C:
            s.Logger(ctx).Info("tick")
        }
    }
}
```

> 网络服务(HTTP/TCP/UDP)无需编写 `Start`：内嵌 `weaver.Listener[H]` 即可由框架自动托管,见[Listener](/guide/listener)。

Start 阶段的行为：

- 所有组件并发启动，全部 Start 已启动后 `weaver.Run` 才进入主逻辑（不阻塞长驻的 Start）
- Start 同步快速失败会中止应用启动并返回错误；异步失败与 panic 会触发整体退出
- 长驻服务应阻塞在 `Start` 内（或使用 `weaver.Listener[H]` 自动托管，见[Listener](/guide/listener)）

## Shutdown

`Shutdown` 在以下时机被调用：

- 主逻辑返回后，`weaver.Run` 退出前统一关闭所有组件
- 收到系统信号（`SIGINT` / `SIGQUIT` / `SIGTERM`）导致 ctx 取消后

```go
func (a *app) Shutdown(ctx context.Context) error {
    a.Logger(ctx).Info("App Shutdown")
    return nil
}
```

## 信号处理与主动退出

`weaver.Run` 内部使用 `signal.NotifyContext` 监听 `SIGINT`、`SIGQUIT`、`SIGTERM`，收到信号后取消上下文。

组件可以通过 `Exec()` 主动触发优雅退出（等价于向进程发送退出信号）：

```go
func (a *app) Init(ctx context.Context) error {
    go func() {
        // 某种满足退出条件时
        a.Exec()
    }()
    return nil
}
```

## 完整流程

```text
weaver.Run / weaver.RunComponent
 ├─ 解析 -conf / -version 参数
 ├─ 加载配置文件
 ├─ 实例化 Main 组件（级联实例化其依赖）
 ├─ 并发调用所有组件的 Start
 ├─ 执行主逻辑 app(ctx, main)
 └─ 退出时统一调用所有组件的 Shutdown
```

`weaver.RunComponent(ctx, (*app)(nil))` 是 `weaver.Run` 的简写形式：启动流程完全一致，只是把主逻辑固定为"等待退出信号"（收到 `SIGINT`/`SIGQUIT`/`SIGTERM` 或组件调用 `Exec()` 后优雅关闭，返回 `nil`）。第二个参数仅用于类型推断。
