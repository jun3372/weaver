package weaver

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/jun3372/weaver/runtime/codegen"
)

type widget struct {
	ctx        context.Context
	conf       *viper.Viper
	regsByName map[string]*codegen.Registration       // registrations by component name
	regsByIntf map[reflect.Type]*codegen.Registration // registrations by component interface type
	regsByImpl map[reflect.Type]*codegen.Registration // registrations by component implementation type
	mu         sync.Mutex                             // guards the following fields
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

	return &widget{
		ctx:        ctx,
		conf:       conf,
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
		return nil, fmt.Errorf("component %v not found; maybe you forgot to run weaver generate", t)
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		// AddSource: true,
	}))

	// slog.Info("logger", "name", name, "attrs", attrs)
	// for _, attr := range attrs {
	// 	logger = logger.With(slog.String("attr", attr))
	// }
	logger = logger.With(slog.String("component", name))

	return logger
}

func (w *widget) get(reg *codegen.Registration) (any, error) {
	if c, ok := w.components[reg.Name]; ok {
		return c, nil
	}

	v := reflect.New(reg.Impl)
	obj := v.Interface()

	// Set logger.
	if err := w.setLogger(obj, w.logger(reg.Name, "x", "1", "b", "2")); err != nil {
		return nil, err
	}

	// todo:: WithConfig
	w.WithConfig(v)
	// todo:: WithRef
	if err := w.WithRef(obj, func(t reflect.Type) (any, error) {
		// reg, ok := w.regsByImpl[t]
		// if !ok {
		// 	return nil, errors.Errorf("component implementation %v not found; maybe you forgot to run weaver generate", t)
		// }

		// return w.get(reg)
		return w.getIntf(t)
	}); err != nil {
		return nil, err
	}

	if i, ok := obj.(interface{ Init(context.Context) error }); ok {
		if err := i.Init(w.ctx); err != nil {
			return nil, fmt.Errorf("component %q initialization failed: %w", reg.Name, err)
		}
	}

	w.components[reg.Name] = obj
	return obj, nil
}

func (w *widget) WithConfig(v reflect.Value) {
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("invalid non pointer to struct value: %v", v))
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
		return fmt.Errorf("WithRefs: %T not a pointer", impl)
	}

	s := p.Elem()
	if s.Kind() != reflect.Struct {
		return fmt.Errorf("WithRefs: %T not a struct pointer", impl)
	}

	for i, n := 0, s.NumField(); i < n; i++ {
		f := s.Field(i)
		if !f.CanAddr() {
			continue
		}

		p := reflect.NewAt(f.Type(), f.Addr().UnsafePointer()).Interface()
		x, ok := p.(interface{ setRef(any) })
		if !ok {
			continue
		}

		valueField := f.Field(0)
		component, err := get(valueField.Type())
		if err != nil {
			return fmt.Errorf("WithRefs: setting field %v.%s: %w", s.Type(), s.Type().Field(i).Name, err)
		}

		if reflect.TypeOf(component).Kind() == reflect.Pointer {
			// component = reflect.ValueOf(component).Elem().Interface()
		}

		x.setRef(component)
	}
	return nil
}

func (w *widget) setLogger(v any, logger *slog.Logger) error {
	x, ok := v.(interface{ setLogger(*slog.Logger) })
	if !ok {
		return fmt.Errorf("setLogger: %T does not implement weaver.Implements", v)
	}
	x.setLogger(logger)
	return nil
}

func (w *widget) start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, impl := range w.components {
		if i, ok := impl.(interface{ Start(context.Context) error }); ok {
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
		if i, ok := impl.(interface{ Shutdown(context.Context) error }); ok {
			if err := i.Shutdown(ctx); err != nil {
				fmt.Printf("Component %s failed to shutdown: %v\n", c, err)
			}
		}
	}
}
