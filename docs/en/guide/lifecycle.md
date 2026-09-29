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

Components implementing `Start` are started **concurrently** before the main logic in `weaver.Run` runs:

```go
func (s *serverImpl) Start(ctx context.Context) error {
    s.Logger(ctx).Info("Starting HTTP server")
    return s.server.ListenAndServe()
}
```

Start-phase behavior:

- All components start concurrently via an errgroup; any failure triggers overall shutdown
- Component panics are recovered and treated as start failures
- A normal return from `Start` means the component finished running (long-running services should block inside `Start`, e.g. `ListenAndServe`)

## Shutdown

`Shutdown` is called:

- After the main logic returns, before `weaver.Run` exits — all components are shut down
- After a system signal (`SIGINT` / `SIGQUIT` / `SIGTERM`) cancels the context
- When a config hot reload restarts components (Shutdown first, then Start)

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
weaver.Run
 ├─ parse -conf / -version flags
 ├─ load config file
 ├─ instantiate the Main component (dependencies cascade)
 ├─ call Start on all components concurrently
 ├─ run the main logic app(ctx, main)
 └─ call Shutdown on all components on exit
```
