# Configuration

Weaver uses [Viper](https://github.com/spf13/viper) for configuration management and supports multiple formats:

- YAML
- TOML
- JSON
- Environment variables

## Injecting Component Configuration

Use the `weaver.WithConfig[T]` generic type together with a struct tag to inject configuration into components. The tags `conf`, `config` and `weaver` are all supported; `conf` is recommended:

```go
type options struct {
    Host string
    Port int
    Auth struct {
        Username string
        Password string
    }
}

type service struct {
    weaver.Implements[Service]
    weaver.WithConfig[options] `conf:"service"` // loaded from the "service" key
}

// Access the config
func (s *service) Init(ctx context.Context) error {
    cfg := s.Config()
    s.Logger(ctx).Info("Service config", "host", cfg.Host, "port", cfg.Port)
    return nil
}
```

The matching config file:

```yaml
service:
  host: 0.0.0.0
  port: 8080
  auth:
    username: admin
    password: secret
```

## System Configuration

The `weaver` key is reserved for framework settings, currently logging:

```yaml
weaver:
  logger:
    level: info
    type: json
    addsource: true
```

See [Logging](/en/guide/logger).

## Hot Reload

Weaver watches the config file via Viper's `WatchConfig`:

- When the file changes, the configuration of all registered components is reloaded automatically (`UnmarshalKey` is re-run) — **no process restart, no service interruption**.
- Whether a value "takes effect" depends on how it is used: fields read via `Config()` on every request/call apply immediately; fields read once inside `Start` (e.g. a server component's listen address) only pick up new values after a restart.

This means saving an updated `weaver.yaml` is enough — no process restart required (as long as the config file was passed via `-conf`).

## Specifying the Config File

Pass it via flag or environment variable:

```bash
go run main.go -conf weaver.yaml
# or
SERVICE_CONFIG=weaver.yaml go run main.go
```

When no config file is specified, `WithConfig` injects nothing.
