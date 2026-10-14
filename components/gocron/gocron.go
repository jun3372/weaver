package gocron

import (
	"context"
	"time"

	gocronv2 "github.com/go-co-op/gocron/v2"
	"github.com/jun3372/weaver"
	"github.com/pkg/errors"
)

type T interface {
	Client(ctx context.Context) (gocronv2.Scheduler, error)
	Register(ctx context.Context, definition gocronv2.JobDefinition, task gocronv2.Task, opts ...gocronv2.JobOption) error
}

type scheduler struct {
	weaver.Implements[T]
	weaver.WithConfig[Config] `conf:"gocron"`
	sched                     gocronv2.Scheduler
}

func (s *scheduler) Client(_ context.Context) (gocronv2.Scheduler, error) {
	if !s.Config().Enabled {
		return nil, ErrNotEnabled
	}

	if s.sched == nil {
		return nil, ErrNotReady
	}

	return s.sched, nil
}

func (s *scheduler) Register(ctx context.Context, definition gocronv2.JobDefinition, task gocronv2.Task, opts ...gocronv2.JobOption) error {
	if !s.Config().Enabled {
		return ErrNotEnabled
	}

	if s.sched == nil {
		return ErrNotReady
	}

	job, err := s.sched.NewJob(definition, task, opts...)
	if err != nil {
		s.Logger(ctx).Error("gocron 任务注册失败", "error", err)
		return err
	}

	s.Logger(ctx).Debug("gocron 任务注册成功", "job", job.Name(), "id", job.ID())
	return nil
}

func (s *scheduler) Init(ctx context.Context) error {
	conf := s.Config()
	if !conf.Enabled {
		s.Logger(ctx).Info("gocron 组件未启用")
		return nil
	}

	s.Logger(ctx).Debug("gocron 组件初始化开始", "config", conf)
	loc, err := timezoneFor(conf.Timezone)
	if err != nil {
		return err
	}

	sched, err := gocronv2.NewScheduler(gocronv2.WithLocation(loc))
	if err != nil {
		s.Logger(ctx).Error("gocron 组件初始化失败", "error", err)
		return err
	}
	sched.Start()

	s.sched = sched
	s.Logger(ctx).Debug("gocron 组件初始化完成", "timezone", loc)
	return nil
}

func (s *scheduler) Shutdown() error {
	if s.sched == nil {
		return nil
	}
	return s.sched.Shutdown()
}

// timezoneFor 解析时区，空串回退 time.Local。
func timezoneFor(name string) (*time.Location, error) {
	if name == "" {
		return time.Local, nil
	}

	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, errors.WithMessage(ErrInvalidTimezone, "timezone: "+name)
	}
	return loc, nil
}
