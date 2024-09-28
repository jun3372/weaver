package weaver

import (
	"context"
	"fmt"
	"log/slog"
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
	regsByImpl map[reflect.Type]*codegen.Registration // registrations by component implementation type
	mu         sync.Mutex                             // guards the following fields
	components map[string]any                         // components, by name
}

func newWidgrt(ctx context.Context, conf *viper.Viper, regs []*codegen.Registration) *widget {
	regsByName := map[string]*codegen.Registration{}
	regsByImpl := map[reflect.Type]*codegen.Registration{}
	for _, reg := range regs {
		regsByName[reg.Name] = reg
		regsByImpl[reg.Impl] = reg
	}

	return &widget{
		ctx:        ctx,
		conf:       conf,
		regsByName: regsByName,
		regsByImpl: regsByImpl,
		components: make(map[string]any),
	}
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

func (w *widget) get(reg *codegen.Registration) (any, error) {
	if c, ok := w.components[reg.Name]; ok {
		return c, nil
	}

	v := reflect.New(reg.Impl)
	obj := v.Interface()

	// todo:: WithConfig
	w.WithConfig(v)
	// todo:: WithRef
	if err := w.WithRef(obj, func(t reflect.Type) (any, error) {
		reg, ok := w.regsByImpl[t]
		if !ok {
			return nil, errors.Errorf("component implementation %v not found; maybe you forgot to run weaver generate", t)
		}

		return w.get(reg)
	}); err != nil {
		return nil, err
	}

	if i, ok := obj.(Init); ok {
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
		return fmt.Errorf("FillRefs: %T not a pointer", impl)
	}

	s := p.Elem()
	if s.Kind() != reflect.Struct {
		return fmt.Errorf("FillRefs: %T not a struct pointer", impl)
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
			return fmt.Errorf("FillRefs: setting field %v.%s: %w", s.Type(), s.Type().Field(i).Name, err)
		}

		tf := reflect.TypeOf(component)
		if tf.Kind() == reflect.Pointer {
			component = reflect.ValueOf(component).Elem().Interface()
		}

		x.setRef(component)
	}
	return nil
}

func (w *widget) shutdown() {
	w.mu.Lock()
	defer w.mu.Unlock()
	ctx := context.Background()
	for c, impl := range w.components {
		if i, ok := impl.(Shutdown); ok {
			if err := i.Shutdown(ctx); err != nil {
				fmt.Printf("Component %s failed to shutdown: %v\n", c, err)
			}
		}
	}
}
