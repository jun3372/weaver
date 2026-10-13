# weaver plugins/gocron

[![Go Reference](https://pkg.go.dev/badge/github.com/jun3372/weaver/plugins/gocron.svg)](https://pkg.go.dev/github.com/jun3372/weaver/plugins/gocron)

Weaver 的定时任务组件，基于 [go-co-op/gocron/v2](https://github.com/go-co-op/gocron)。声明 `weaver.Ref[gocron.T]` 即可注入，scheduler 的创建、启动与优雅关闭由框架生命周期托管，使用方只需注册任务。

## 安装

```bash
go get github.com/jun3372/weaver/plugins/gocron
```

在主包匿名导入以完成组件注册：

```go
import _ "github.com/jun3372/weaver/plugins/gocron"
```

## 配置

```yaml
gocron:
  Enabled: true
  Timezone: "Asia/Shanghai"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用，未启用时组件注入返回 `ErrNotEnabled` |
| `Timezone` | 空 | IANA 时区名（如 `Asia/Shanghai`），留空取本地时区 |

环境变量覆盖：`WEAVER_GOCRON_ENABLED`、`WEAVER_GOCRON_TIMEZONE`。

## 使用示例

```go
type reportService struct {
    weaver.Implements[ReportService]
    weaver.Ref[gocron.T]
}

func (s *reportService) Init(ctx context.Context) error {
    return s.Ref().Register(ctx,
        gocronv2.CronJob("0 9 * * *", false), // 每天 9 点
        gocronv2.NewTask(s.dailyReport),
        gocronv2.WithName("daily-report"),
    )
}

func (s *reportService) dailyReport() {
    // ...
}
```

需要更高自由度（按 tag 查询/删除任务、单例模式、任务监听器等）时，可通过 `Client(ctx)` 拿到原生 `gocronv2.Scheduler` 自行操作。

## 生命周期

- `Init`：未启用时打日志直接返回；解析 `Timezone` 后 `NewScheduler` 创建 scheduler（`WithLocation`），随即 `Start`——weaver 按依赖序初始化，使用方组件的 `Init` 在其后执行，此时注册的任务会被立即调度
- `Shutdown`：`scheduler.Shutdown()` 停止全部任务并释放资源；对未初始化（未启用）的组件是安全的

## 完整教程

更多用法见文档站的[任务调度教程](https://github.com/jun3372/weaver/blob/main/docs/guide/plugins/scheduler.md#gocron)。
