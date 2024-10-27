package weaver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/sync/singleflight"

	"github.com/jun3372/weaver/internal/config"
	"github.com/jun3372/weaver/runtime/codegen"
)

type widget struct {
	ctx        context.Context
	conf       *viper.Viper
	config     *config.Config
	mu         sync.Mutex // guards the following fields
	single     singleflight.Group
	regsByName map[string]*codegen.Registration       // registrations by component name
	regsByIntf map[reflect.Type]*codegen.Registration // registrations by component interface type
	regsByImpl map[reflect.Type]*codegen.Registration // registrations by component implementation type
	components map[string]any                         // components, by name
}

func newWidgrt(ctx context.Context, conf *viper.Viper, regs []*codegen.Registration) *widget {
	regsByName := map[string]*codegen.Registration{}
	regsByIntf := map[reflect.Type]*codegen.Registration{}
	regsByImpl := map[reflect.Type]*codegen.Registration{}
	for _, reg := range regs {
		regsByName[reg.Name] = reg
		regsByIntf[reg.Iface] = reg
		regsByImpl[reg.Impl] = reg
	}

	var config = new(config.Config)
	if len(conf.AllKeys()) > 0 {
		if err := conf.UnmarshalKey("weaver", &config); err != nil {
			slog.Warn("failed to unmarshal system config", "err", err)
		}
	}

	return &widget{
		ctx:        ctx,
		conf:       conf,
		config:     config,
		regsByName: regsByName,
		regsByIntf: regsByIntf,
		regsByImpl: regsByImpl,
		components: make(map[string]any),
	}
}

func (w *widget) GetIntf(t reflect.Type) (any, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.getIntf(t)
}

func (w *widget) getIntf(t reflect.Type) (any, error) {
	reg, ok := w.regsByIntf[t]
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
	var wr io.Writer
	var level = slog.LevelInfo

	switch strings.ToUpper(w.config.Logger.Level) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	}

	wr = os.Stderr
	source := w.config.Logger.AddSource
	if w.config.Logger.File != "" {
		// todo:: 打开文件
	}

	var handler slog.Handler
	opts := slog.HandlerOptions{Level: level, AddSource: source}
	if w.config != nil && strings.ToLower(w.config.Logger.Type) == "json" {
		handler = slog.NewJSONHandler(wr, &opts)
	} else {
		handler = slog.NewTextHandler(wr, &opts)
	}

	return slog.New(handler).With(slog.String("component", name))
}

func (w *widget) get(reg *codegen.Registration) (any, error) {
	if c, ok := w.components[reg.Name]; ok {
		return c, nil
	}

	v := reflect.New(reg.Impl)
	obj := v.Interface()

	// Set logger.
	if err := w.setLogger(obj, w.logger(reg.Name)); err != nil {
		return nil, err
	}

	// WithConfig
	if len(w.conf.AllKeys()) > 0 {
		w.WithConfig(v)
	}

	// WithRef
	if err := w.WithRef(obj, func(t reflect.Type) (any, error) { return w.getIntf(t) }); err != nil {
		return nil, err
	}

	if i, ok := obj.(interface{ Init(_ context.Context) error }); ok {
		_, err, _ := w.single.Do("init."+reg.Name, func() (any, error) {
			err := i.Init(w.ctx)
			return nil, err
		})

		if err != nil {
			return nil, errors.Errorf("component %q initialization failed: %v", reg.Name, err)
		}
	}

	_, err, _ := w.single.Do("start."+reg.Name, func() (any, error) {
		err := w.start(w.ctx)
		return nil, err
	})

	if err != nil {
		return nil, errors.Errorf("component %q startup failed: %v", reg.Name, err)
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

		key := f.Tag.Get("weaver")
		if f.Anonymous {
			config := s.Field(i).Addr().MethodByName("Config")
			cfg := config.Call(nil)[0].Interface()
			if err := w.conf.UnmarshalKey(key, cfg); err != nil {
				slog.Warn("解析配置信息出错", slog.String("key", key), slog.Any("cfg", cfg), slog.Any("err", err))
			}
			continue
		}

		// todo:: f.Anonymous == false 使用 w.conf.UnmarshalKey 方法给这个f设置值
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

func (w *widget) setLogger(v any, logger *slog.Logger) error {
	x, ok := v.(interface{ setLogger(_ *slog.Logger) })
	if !ok {
		return errors.Errorf("setLogger: %T does not implement weaver.Implements", v)
	}

	x.setLogger(logger)
	return nil
}

func (w *widget) start(ctx context.Context) error {
	for _, impl := range w.components {
		if i, ok := impl.(interface{ Start(_ context.Context) error }); ok {
			if err := i.Start(ctx); err != nil {
				return err
			}
		}
	}
	return nil
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
