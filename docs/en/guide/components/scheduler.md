# Task Scheduling: gocron

## gocron

Weaver's scheduled-task component, built on [go-co-op/gocron/v2](https://github.com/go-co-op/gocron). Declare a `weaver.Ref[gocron.T]` to inject it; scheduler creation, startup and graceful shutdown are managed by the framework lifecycle, so consumers only register jobs. gocron is not a client that connects to an external service — there is no DSN or health probe.

### Installation

```bash
go get github.com/jun3372/weaver/components/gocron
```

```go
import _ "github.com/jun3372/weaver/components/gocron" // blank import completes component registration
```

### Configuration

```yaml
gocron:
  Enabled: true
  Timezone: "Asia/Shanghai"
```

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether the component is enabled; when disabled, injection calls return `ErrNotEnabled` |
| `Timezone` | empty | IANA timezone name (e.g. `Asia/Shanghai`); empty means the local timezone |

Environment variable overrides: `WEAVER_GOCRON_ENABLED`, `WEAVER_GOCRON_TIMEZONE`.

### Basic Usage

The component's `Init` creates and starts the scheduler (weaver initializes in dependency order, so the consumer component's `Init` runs afterwards, and jobs registered at that point are scheduled immediately). Register jobs via `Register` in your own `Init`:

```go
type reportService struct {
    weaver.Implements[ReportService]
    weaver.Ref[gocron.T]
}

func (s *reportService) Init(ctx context.Context) error {
    return s.Get().Register(ctx,
        gocronv2.CronJob("0 9 * * *", false), // every day at 9:00; second arg withSeconds=false
        gocronv2.NewTask(s.dailyReport),
        gocronv2.WithName("daily-report"),
    )
}

func (s *reportService) dailyReport() {
    // ...
}
```

Both the job definition and the execution body are native gocron types: `gocronv2.NewTask(fn, params...)` wraps any function, `CronJob` / `DailyJob` / `DurationJob` describe the trigger, and JobOptions such as `WithName` / `WithSingletonMode` fine-tune behavior.

### Advanced Scenarios

#### Fixed Interval and Daily at Time

```go
// Every 30 minutes
_ = s.Get().Register(ctx,
    gocronv2.DurationJob(30*time.Minute),
    gocronv2.NewTask(s.syncData),
)

// Every day at 02:30
_ = s.Get().Register(ctx,
    gocronv2.DailyJob(1, gocronv2.NewAtTimes(gocronv2.NewAtTime(2, 30, 0))),
    gocronv2.NewTask(s.cleanup),
)
```

#### Singleton Mode (Preventing Overlap)

```go
// Skip a trigger while the previous run is still executing
_ = s.Get().Register(ctx,
    gocronv2.DurationJob(time.Minute),
    gocronv2.NewTask(s.longPolling),
    gocronv2.WithSingletonMode(gocron.LimitModeReschedule),
)
```

#### Job Context Cancellation

Declare `context.Context` as the first parameter of the function passed to `NewTask` to receive the job context; it is canceled when the scheduler shuts down — listen to it in long-running jobs for graceful exit:

```go
_ = s.Get().Register(ctx,
    gocronv2.DurationJob(time.Minute),
    gocronv2.NewTask(s.watch),
)

func (s *reportService) watch(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return // reached when the scheduler shuts down
        default:
            // ...
        }
    }
}
```

#### Managing Jobs via the Native Scheduler

To look up/remove jobs by tag or adjust dynamically, get the raw `gocronv2.Scheduler` via `Client(ctx)`:

```go
sched, err := s.Get().Client(ctx)
if err != nil {
    return err
}
for _, job := range sched.Jobs() {
    _ = job.Name() // match, then sched.RemoveJob(job.ID())
}
```

### Lifecycle & Errors

- `Init`: if not enabled, logs and returns immediately; parses `Timezone`, creates the scheduler with `NewScheduler` (plus `WithLocation`) and calls `Start` (`Start` is non-blocking; startup failures are not surfaced here)
- `Shutdown`: `scheduler.Shutdown()` stops all jobs and releases resources; safe on uninitialized (disabled) components
- Common errors: `ErrNotEnabled`/`ErrNotReady` (component not enabled / not ready), `ErrInvalidTimezone` (`Timezone` is not a valid IANA name); `Register` returns gocron's parse error for invalid job definitions (e.g. a bad crontab)

## Further Reading

- [Components Overview](/en/guide/components)
- [Configuration](/en/guide/config)
