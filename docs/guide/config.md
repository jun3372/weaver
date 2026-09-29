# 配置管理

Weaver 使用 [Viper](https://github.com/spf13/viper) 进行配置管理，支持多种配置格式：

- YAML
- TOML
- JSON
- 环境变量

## 注入组件配置

通过 `weaver.WithConfig[T]` 泛型类型和结构体标签，可以将配置自动注入到组件中。标签支持 `conf`、`config`、`weaver` 三种写法，推荐使用 `conf`：

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
    weaver.WithConfig[options] `conf:"service"` // 从配置中的 "service" 键加载
}

// 访问配置
func (s *service) Init(ctx context.Context) error {
    cfg := s.Config() // 获取配置
    s.Logger(ctx).Info("Service config", "host", cfg.Host, "port", cfg.Port)
    return nil
}
```

对应的配置文件：

```yaml
service:
  host: 0.0.0.0
  port: 8080
  auth:
    username: admin
    password: secret
```

## 系统配置

`weaver` 键是框架保留的系统配置，当前支持日志设置：

```yaml
weaver:
  logger:
    level: info
    type: json
    addsource: true
```

详见[日志系统](/guide/logger)。

## 配置热更新

Weaver 通过 Viper 的 `WatchConfig` 监听配置文件变更：

- 配置文件发生变化时，所有已注册的组件配置会自动重新加载（重新执行 `UnmarshalKey`）。
- 同时，框架会先执行所有组件的 `Shutdown`，再重新 `Start`，实现不重启进程的配置生效。

这意味着修改 `weaver.yaml` 保存后，无需重启应用即可让新配置生效（前提是以 `-conf` 指定了配置文件路径）。

## 指定配置文件

通过启动参数或环境变量指定：

```bash
go run main.go -conf weaver.yaml
# 或
SERVICE_CONFIG=weaver.yaml go run main.go
```

未指定配置文件时，`WithConfig` 不会注入任何值。
