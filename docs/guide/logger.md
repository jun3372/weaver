# 日志系统

Weaver 使用 Go 标准库的 `log/slog` 提供结构化日志，并通过 [lumberjack](https://github.com/natefinch/lumberjack) 支持日志文件轮转。

## 在组件中使用日志

所有嵌入了 `weaver.Implements[T]` 的组件都可以通过 `Logger(ctx)` 获取日志器：

```go
func (a *app) DoSomething(ctx context.Context) {
    logger := a.Logger(ctx) // 获取日志器

    logger.Info("Processing request", "requestID", "12345")
    logger.Warn("Resource running low", "resource", "memory", "available", "10%")
    logger.Error("Operation failed", "error", errors.New("connection timeout"))
}
```

## 与链路追踪自动关联

`Logger(ctx)` 会自动检查上下文中的 OpenTelemetry Span：

- 上下文包含有效 TraceID 时，日志自动附带 `trace_id` 字段
- 上下文包含有效 SpanID 时，日志自动附带 `span_id` 字段

这样每条业务日志都能与分布式链路关联起来。详见 [OpenTelemetry 集成](/guide/opentelemetry)。

## 日志配置

```yaml
weaver:
  logger:
    level: info       # 日志级别：debug, info, warn, error
    type: json        # 日志格式：json 或 text（console）
    addsource: true   # 是否添加源代码位置
    file:
      filename: "./logs/app.log"  # 日志文件路径
      maxsize: 100               # 单个日志文件最大大小(MB)
      maxage: 7                  # 日志文件保留天数
      maxbackups: 10             # 保留的旧日志文件数量
      compress: true             # 是否压缩旧日志
      localtime: true            # 使用本地时间
```

## 配置项说明

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `level` | `info` | 日志级别，无效值会回退为 `info` |
| `type` | `console` | 输出格式：`json` / `text` / `console` |
| `addsource` | `false` | 日志是否携带源码位置 |
| `file.filename` | 空 | 为空时仅输出到标准输出 |
| `file.maxsize` | `100` | 单文件最大大小（MB），超过后轮转 |
| `file.maxage` | `7` | 旧日志保留天数 |
| `file.maxbackups` | `3` | 保留的旧日志文件数量 |
| `file.compress` | `true` | 轮转后是否 gzip 压缩 |
| `file.localtime` | `true` | 时间戳是否使用本地时间 |

日志会同时写入标准输出和配置的文件（MultiWriter）。
