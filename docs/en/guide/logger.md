# Logging

Weaver provides structured logging through the Go standard library `log/slog`, with rotating log files via [lumberjack](https://github.com/natefinch/lumberjack).

## Using Logs in Components

Every component embedding `weaver.Implements[T]` can obtain a logger via `Logger(ctx)`:

```go
func (a *app) DoSomething(ctx context.Context) {
    logger := a.Logger(ctx)

    logger.Info("Processing request", "requestID", "12345")
    logger.Warn("Resource running low", "resource", "memory", "available", "10%")
    logger.Error("Operation failed", "error", errors.New("connection timeout"))
}
```

## Automatic Trace Correlation

`Logger(ctx)` inspects the OpenTelemetry span in the context:

- When the context carries a valid TraceID, logs automatically include a `trace_id` field
- When the context carries a valid SpanID, logs automatically include a `span_id` field

Every business log line can thus be tied to its distributed trace. See [OpenTelemetry Integration](/en/guide/opentelemetry).

## Log Configuration

```yaml
weaver:
  logger:
    level: info       # level: debug, info, warn, error
    type: json        # format: json or text (console)
    addsource: true   # include source code location
    file:
      filename: "./logs/app.log"  # log file path
      maxsize: 100               # max size of a single file (MB)
      maxage: 7                  # days to retain old logs
      maxbackups: 10             # number of old files to keep
      compress: true             # gzip rotated files
      localtime: true            # use local time
```

## Options Reference

| Option | Default | Description |
| --- | --- | --- |
| `level` | `info` | Log level; invalid values fall back to `info` |
| `type` | `console` | Output format: `json` / `text` / `console` |
| `addsource` | `false` | Attach source location to log entries |
| `file.filename` | empty | When empty, logs go to stdout only |
| `file.maxsize` | `100` | Max size of a single file (MB) before rotation |
| `file.maxage` | `7` | Days to retain rotated files |
| `file.maxbackups` | `3` | Number of rotated files to keep |
| `file.compress` | `true` | Gzip-compress rotated files |
| `file.localtime` | `true` | Use local time for timestamps |

Logs are written to stdout and the configured file simultaneously (MultiWriter).
