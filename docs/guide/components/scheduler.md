# 任务调度:gocron

## gocron

Weaver 的定时任务组件,基于 [go-co-op/gocron/v2](https://github.com/go-co-op/gocron)。声明 `weaver.Ref[gocron.T]` 即可注入,scheduler 的创建、启动与优雅关闭由框架生命周期托管,使用方只需注册任务。gocron 不是连接外部服务的客户端,无需 DSN 与探活。

### 安装

```bash
go get github.com/jun3372/weaver/components/gocron
```

```go
import _ "github.com/jun3372/weaver/components/gocron" // 匿名导入完成组件注册
```

### 配置

```yaml
gocron:
  Enabled: true
  Timezone: "Asia/Shanghai"
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `Enabled` | `false` | 是否启用,未启用时注入调用返回 `ErrNotEnabled` |
| `Timezone` | 空 | IANA 时区名(如 `Asia/Shanghai`),留空取本地时区 |

环境变量覆盖:`WEAVER_GOCRON_ENABLED`、`WEAVER_GOCRON_TIMEZONE`。

### 基础用法

组件 `Init` 阶段即创建并启动 scheduler(weaver 按依赖序初始化,使用方组件的 `Init` 在其后执行,此时注册的任务会被立即调度)。在自己的 `Init` 里经 `Register` 注册任务:

```go
type reportService struct {
    weaver.Implements[ReportService]
    weaver.Ref[gocron.T]
}

func (s *reportService) Init(ctx context.Context) error {
    return s.Get().Register(ctx,
        gocronv2.CronJob("0 9 * * *", false), // 每天 9 点;第二参数 withSeconds=false
        gocronv2.NewTask(s.dailyReport),
        gocronv2.WithName("daily-report"),
    )
}

func (s *reportService) dailyReport() {
    // ...
}
```

任务定义与执行体均为 gocron 原生类型:`gocronv2.NewTask(函数, 参数...)` 包装任意函数,`CronJob` / `DailyJob` / `DurationJob` 等描述触发时机,`WithName` / `WithSingletonMode` 等 JobOption 微调行为。

### 进阶场景

#### 固定间隔与每天定时

```go
// 每 30 分钟一次
_ = s.Get().Register(ctx,
    gocronv2.DurationJob(30*time.Minute),
    gocronv2.NewTask(s.syncData),
)

// 每天凌晨 2:30
_ = s.Get().Register(ctx,
    gocronv2.DailyJob(1, gocronv2.NewAtTimes(gocronv2.NewAtTime(2, 30, 0))),
    gocronv2.NewTask(s.cleanup),
)
```

#### 单例模式(防止任务重入)

```go
// 上一次还没跑完时跳过本次触发
_ = s.Get().Register(ctx,
    gocronv2.DurationJob(time.Minute),
    gocronv2.NewTask(s.longPolling),
    gocronv2.WithSingletonMode(gocron.LimitModeReschedule),
)
```

#### 任务上下文取消

`NewTask` 的函数第一个参数声明为 `context.Context` 即可收到任务上下文,scheduler 关闭时会被取消——长任务里监听它实现优雅退出:

```go
_ = s.Get().Register(ctx,
    gocronv2.DurationJob(time.Minute),
    gocronv2.NewTask(s.watch),
)

func (s *reportService) watch(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return // scheduler Shutdown 时走到这里
        default:
            // ...
        }
    }
}
```

#### 通过原生 Scheduler 管理

需要按 tag 查询/移除任务、动态调整时,经 `Client(ctx)` 拿到 `gocronv2.Scheduler` 自行操作:

```go
sched, err := s.Get().Client(ctx)
if err != nil {
    return err
}
for _, job := range sched.Jobs() {
    _ = job.Name() // 匹配后 sched.RemoveJob(job.ID())
}
```

### 生命周期与错误

- `Init`:未启用时打日志直接返回;解析 `Timezone` 后 `NewScheduler`(带 `WithLocation`)创建 scheduler 并 `Start`(`Start` 非阻塞,启动失败不会在此暴露)
- `Shutdown`:`scheduler.Shutdown()` 停止全部任务并释放资源;对未初始化(未启用)的组件安全
- 常见错误:`ErrNotEnabled`/`ErrNotReady`(组件未启用/未就绪)、`ErrInvalidTimezone`(`Timezone` 不是合法 IANA 名称);`Register` 对非法任务定义(如错误的 crontab)会返回 gocron 的解析错误

## 相关阅读

- [组件总览](/guide/components)
- [配置管理](/guide/config)
