package weaver

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"unsafe"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"

	"github.com/jun3372/weaver/internal/config"
	"github.com/jun3372/weaver/runtime/codegen"
	"github.com/jun3372/weaver/runtime/logger"
)

var (
	once sync.Once
	log  *slog.Logger
)

type widget struct {
	ctx             context.Context
	conf            *viper.Viper
	option          *config.Config
	mu              sync.Mutex
	cancel          context.CancelFunc
	regsByName      map[string]*codegen.Registration       // registrations by component name
	regsByInterface map[reflect.Type]*codegen.Registration // registrations by component interface type
	regsByImpl      map[reflect.Type]*codegen.Registration // registrations by component implementation type
	components      map[string]any                         // components, by name
	watchConfig     []func()
}

func newWidget(ctx context.Context, cancel context.CancelFunc, conf *viper.Viper, regs []*codegen.Registration) *widget {
	w := widget{
		ctx:             ctx,
		conf:            conf,
		cancel:          cancel,
		option:          new(config.Config),
		regsByName:      map[string]*codegen.Registration{},
		regsByInterface: map[reflect.Type]*codegen.Registration{},
		regsByImpl:      map[reflect.Type]*codegen.Registration{},
		components:      make(map[string]any),
		watchConfig:     []func(){},
	}

	for _, reg := range regs {
		w.regsByName[reg.Name] = reg
		w.regsByImpl[reg.Impl] = reg
		w.regsByInterface[reg.Interface] = reg
	}

	if w.conf != nil {
		if err := conf.UnmarshalKey("weaver", &w.option); err != nil {
			slog.Warn("failed to unmarshal system config", "err", err)
		}

		conf.WatchConfig()
		conf.OnConfigChange(func(e fsnotify.Event) {
			for _, fn := range w.watchConfig {
				fn()
			}

			w.shutdown(context.Background())
			w.start(context.Background())
		})
	}

	return &w
}

func (w *widget) GetInterface(t reflect.Type) (any, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.getInterface(t)
}

func (w *widget) getInterface(t reflect.Type) (any, error) {
	reg, ok := w.regsByInterface[t]
	if !ok {
		return nil, errors.Errorf("component %v not found; maybe you forgot to run weaver generate", t)
	}

	c, err := w.get(reg)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (w *widget) getImpl(t reflect.Type) (any, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	reg, ok := w.regsByImpl[t]
	if !ok {
		return nil, errors.Errorf("component implementation %v not found; maybe you forgot to run weaver generate", t)
	}

	return w.get(reg)
}

func (w *widget) logger(name string, attrs ...string) *slog.Logger {
	once.Do(func() {
		opts := []logger.Option{
			logger.WithType(w.option.Logger.Type),
			logger.WithLevelString(w.option.Logger.Level),
			logger.WithAddSource(w.option.Logger.AddSource),
		}

		if w.option.Logger.File != nil {
			opts = append(opts, []logger.Option{
				logger.WithCompress(w.option.Logger.File.Compress),
				logger.WithFilename(w.option.Logger.File.Filename),
				logger.WithMaxAge(w.option.Logger.File.MaxAge),
				logger.WithMaxBackups(w.option.Logger.File.MaxBackups),
				logger.WithMaxSize(w.option.Logger.File.MaxSize),
				logger.WithLocalTime(w.option.Logger.File.LocalTime),
			}...)
		}

		log = logger.New(opts...).Logger(context.Background())
	})

	return log
}

func (w *widget) get(reg *codegen.Registration) (any, error) {
	if c, ok := w.components[reg.Name]; ok {
		return c, nil
	}

	v := reflect.New(reg.Impl)
	obj := v.Interface()

	// 设置中途退出方法
	if w.cancel != nil {
		if i, ok := obj.(interface{ setExec(context.CancelFunc) }); ok {
			i.setExec(w.cancel)
		}
	}

	// Set logger.
	if err := w.setLogger(obj, w.logger(reg.Name)); err != nil {
		return nil, err
	}

	// WithConfig
	if w.conf != nil {
		w.WithConfig(v)
	}

	// WithRef
	if err := w.WithRef(obj, func(t reflect.Type) (any, error) { return w.getInterface(t) }); err != nil {
		return nil, err
	}

	if i, ok := obj.(interface{ Init(_ context.Context) error }); ok {
		if err := i.Init(w.ctx); err != nil {
			return nil, errors.Errorf("component %q initialization failed: %v", reg.Name, err)
		}
	}

	w.components[reg.Name] = obj
	return obj, nil
}

func (w *widget) WithConfig(v reflect.Value) {
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		panic(errors.Errorf("invalid non pointer to struct value: %v", v))
	}

	s := v.Elem()
	t := s.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !strings.HasPrefix(f.Type.Name(), "WithConfig[") {
			continue
		}

		var key string
		for _, v := range config.Tags() {
			if key = f.Tag.Get(v); key != "" {
				break
			}
		}

		if key == "" {
			w.logger("weaver").Info("未找到配置依赖标签", "struct", t, "fieldName", f.Name, "fieldType", f.Type, "tag", f.Tag)
			continue
		}

		if f.Anonymous {
			configGetter := s.Field(i).Addr().MethodByName("Config").Call(nil)[0]
			if err := w.conf.UnmarshalKey(key, configGetter.Interface()); err != nil {
				w.logger("weaver").Error("解析配置失败", "key", key, "err", err)
				continue
			}

			w.WatchConfig(key, func() {
				if err := w.conf.UnmarshalKey(key, configGetter.Interface()); err != nil {
					w.logger("weaver").Error("解析配置失败", "key", key, "err", err)
				}
			})
			continue
		}

		// f.Anonymous == false 使用 w.conf.UnmarshalKey 方法给这个f设置值
		field := s.FieldByIndex(f.Index)
		if !field.CanAddr() {
			continue
		}

		fieldValue := reflect.New(field.Type()).Elem()
		configField := fieldValue.Addr().MethodByName("Config")
		if !configField.IsValid() {
			w.logger("weaver").Warn("未找到 Config 字段", slog.String("key", key), slog.Any("field", field))
			continue
		}

		cfg := configField.Call(nil)[0].Interface()
		if err := w.conf.UnmarshalKey(key, cfg); err != nil {
			w.logger("weaver").Warn("解析配置信息出错", slog.String("key", key), slog.Any("configField", cfg), slog.Any("err", err))
			continue
		}

		// 使用反射设置未导出字段的值
		reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(fieldValue)

		// 监听配置变化
		w.WatchConfig(key, func() {
			if err := w.conf.UnmarshalKey(key, cfg); err != nil {
				w.logger("weaver").Warn("解析配置信息出错", slog.String("key", key), slog.Any("configField", cfg), slog.Any("err", err))
				return
			}

			// 使用反射设置未导出字段的值
			reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(fieldValue)
		})
	}
}

func (w *widget) WithRef(impl any, get func(t reflect.Type) (any, error)) error {
	p := reflect.ValueOf(impl)
	if p.Kind() != reflect.Pointer {
		return errors.Errorf("WithRefs: %T not a pointer", impl)
	}

	s := p.Elem()
	if s.Kind() != reflect.Struct {
		return errors.Errorf("WithRefs: %T not a struct pointer", impl)
	}

	for i, n := 0, s.NumField(); i < n; i++ {
		f := s.Field(i)
		if !f.CanAddr() {
			continue
		}

		p := reflect.NewAt(f.Type(), f.Addr().UnsafePointer()).Interface()
		x, ok := p.(interface{ setRef(_ any) })
		if !ok {
			continue
		}

		valueField := f.Field(0)
		component, err := get(valueField.Type())
		if err != nil {
			return errors.Errorf("WithRefs: setting field %v.%s: %v", s.Type(), s.Type().Field(i).Name, err)
		}

		if reflect.TypeOf(component).Kind() == reflect.Pointer {
			// component = reflect.ValueOf(component).Elem().Interface()
		}

		x.setRef(component)
	}
	return nil
}

func (w *widget) WatchConfig(key string, fn func()) {
	// 初始化切片
	if w.watchConfig == nil {
		// w.mu.Lock()
		// defer w.mu.Unlock()
		w.watchConfig = make([]func(), 0)
	}

	w.watchConfig = append(w.watchConfig, fn)
}

func (w *widget) setLogger(v any, logger *slog.Logger) error {
	x, ok := v.(interface{ setLogger(_ *slog.Logger) })
	if !ok {
		return errors.Errorf("setLogger: %T does not implement weaver.Implements", v)
	}

	x.setLogger(logger)
	return nil
}

func (w *widget) start(ctx context.Context) error {
	var wg *errgroup.Group
	wg, ctx = errgroup.WithContext(ctx)
	for _, impl := range w.components {
		if i, ok := impl.(interface{ Start(_ context.Context) error }); ok {
			go wg.Go(func() error {
				var err error
				defer func() {
					if e := recover(); e != nil {
						log.Error("Component startup encountered an exception", "err", err, "e", e)
						w.cancel()
						if err == nil {
							err = e.(error)
						}

						return
					}
				}()

				if err = i.Start(ctx); err != nil {
					log.Error("Component startup failed", "err", err)
					w.cancel()
				}

				return err
			})
		}
	}

	return wg.Wait()
}

func (w *widget) shutdown(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for c, impl := range w.components {
		if i, ok := impl.(interface{ Shutdown(_ context.Context) error }); ok {
			if err := i.Shutdown(ctx); err != nil {
				fmt.Printf("Component %s failed to shutdown: %v\n", c, err)
			}
		}
	}
}
