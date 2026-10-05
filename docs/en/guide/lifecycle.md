# Lifecycle Management

Weaver components support the following optional lifecycle hooks:

| Hook | Signature | Description |
| --- | --- | --- |
| Init | `Init(ctx context.Context) error` | Called on component initialization; returning an error aborts startup |
| Start | `Start(ctx context.Context) error` | Called when the component starts; suitable for long-running services |
| Shutdown | `Shutdown(ctx context.Context) error` | Called on component shutdown to release resources |

## Init

`Init` is called right after the component is instantiated. Dependency injection (`WithConfig`, `Ref`) has already completed, so config and dependencies are safe to access.

## Start

Components implementing `Start` are started **concurrently** before the main logic in `weaver.Run` runs. Ideal for long-running background jobs:

```go
func (s *worker) Start(ctx context.Context) error {
    t := time.NewTicker(time.Second)
    defer t.Stop()
    for {
        select {
        case <-ctx.Done():
            return nil // context done, job exits
        case <-t.C:
            s.Logger(ctx).Info("tick")
        }
    }
}
```

> Network servers (HTTP/TCP/UDP) need no `Start`: embed `weaver.Listener[H]` and the framework hosts them automatically, see [Listener](/en/guide/listener).

Start-phase behavior:

- All components start concurrently; `weaver.Run` proceeds to the main logic once every `Start` has been launched (long-running `Start`s don't block startup)
- A synchronous fast failure in `Start` aborts startup and returns the error; async failures and panics trigger overall shutdown
- Long-running services should block inside `Start` (or use `weaver.Listener[H]` for automatic hosting, see [Listener](/en/guide/listener))

## Shutdown

`Shutdown` is called:

- After the main logic returns, before `weaver.Run` exits — all components are shut down
- After a system signal (`SIGINT` / `SIGQUIT` / `SIGTERM`) cancels the context

```go
func (a *app) Shutdown(ctx context.Context) error {
    a.Logger(ctx).Info("App Shutdown")
    return nil
}
```

## Signals and Graceful Exit

`weaver.Run` listens for `SIGINT`, `SIGQUIT` and `SIGTERM` via `signal.NotifyContext` and cancels the context on receipt.

Components can trigger graceful shutdown proactively via `Exec()` (equivalent to sending the process an exit signal):

```go
func (a *app) Init(ctx context.Context) error {
    go func() {
        // when some exit condition is met
        a.Exec()
    }()
    return nil
}
```

## Complete Flow

```text
weaver.Run / weaver.RunComponent
 ├─ parse -conf / -version flags
 ├─ load config file
 ├─ instantiate the Main component (dependencies cascade)
 ├─ call Start on all components concurrently
 ├─ run the main logic app(ctx, main)
 └─ call Shutdown on all components on exit
```

`weaver.RunComponent(ctx, (*app)(nil))` is a shorthand for `weaver.Run` with an identical startup flow — the main logic is fixed to "wait for the exit signal" (graceful shutdown on `SIGINT`/`SIGQUIT`/`SIGTERM` or a component calling `Exec()`, returning `nil`). The second parameter exists only for type inference.
