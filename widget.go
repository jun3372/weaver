package weaver

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/jun3372/weaver/internal/config"
	"github.com/jun3372/weaver/runtime/codegen"
	"github.com/jun3372/weaver/runtime/logger"
)

type widget struct {
	ctx             context.Context
	conf            *viper.Viper
	option          *config.Config
	log             *slog.Logger
	mu              sync.Mutex
	watchMu         sync.Mutex
	cancel          context.CancelFunc
	startWG         sync.WaitGroup                         // 等待所有组件 Start 返回
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
	}
	w.log = w.newLogger()
	if w.conf == nil {
		w.log.Info("未指定配置文件(通过 -conf 参数或 SERVICE_CONFIG 环境变量),组件配置将不会被注入")
	} else {
		conf.WatchConfig()
		conf.OnConfigChange(func(e fsnotify.Event) {
			w.log.Info("检测到配置变更", "file", e.Name)
			w.reloadConfig()
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

func (w *widget) newLogger() *slog.Logger {
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

	return logger.New(opts...).Logger(context.Background())
}

// logger 返回组件 logger;开启 Component 选项时会附加组件名属性。
func (w *widget) logger(name string) *slog.Logger {
	if w.option.Logger.Component {
		return w.log.With("component", name)
	}
	return w.log
}

func (w *widget) get(reg *codegen.Registration) (any, error) {
	if c, ok := w.components[reg.Name]; ok {
		return c, nil
	}

	v := reflect.New(reg.Impl)
	obj := v.Interface()

	// 先注册再装配 Ref,这样组件间循环引用会解析到同一实例而不是无限递归
	w.components[reg.Name] = obj

	// 设置中途退出方法
	if w.cancel != nil {
		if i, ok := obj.(interface{ setExec(context.CancelFunc) }); ok {
			i.setExec(w.cancel)
		}
	}

	// Set logger.
	componentLog := w.logger(reg.Name)
	if err := w.setLogger(obj, componentLog); err != nil {
		w.deregister(reg.Name)
		return nil, err
	}

	// WithConfig:配置注入与 Listener 装配
	if err := w.WithConfig(v, obj, componentLog); err != nil {
		w.deregister(reg.Name)
		return nil, err
	}

	// WithRef
	if err := w.WithRef(obj, func(t reflect.Type) (any, error) { return w.getInterface(t) }); err != nil {
		w.deregister(reg.Name)
		return nil, err
	}

	if i, ok := obj.(interface{ Init(_ context.Context) error }); ok {
		if err := i.Init(w.ctx); err != nil {
			w.deregister(reg.Name)
			return nil, errors.Errorf("component %q initialization failed: %v", reg.Name, err)
		}
	}

	return obj, nil
}

// deregister 清理注册失败的组件;调用方必须已持有 w.mu(get 路径)。
func (w *widget) deregister(name string) {
	delete(w.components, name)
}

// isConfigManagedType 判断字段类型是否为框架托管配置的类型:
// WithConfig[T] 组件,具备 unexported config 字段与 SetConfig 方法,共用注入链路。
// (HTTP/TCP/UDP 服务已收敛为 Listener[H],其配置注入由 initListener 单独处理。)
func isConfigManagedType(name string) bool {
	return strings.HasPrefix(name, "WithConfig[")
}

// fieldConfTag 读取字段的配置依赖标签(conf/weaver/config)。
func fieldConfTag(f reflect.StructField) string {
	for _, tag := range config.Tags() {
		if key := f.Tag.Get(tag); key != "" {
			return key
		}
	}
	return ""
}

// listenerAPI 是 widget 装配与启动 Listener 字段所需的同包接口
// (方法为 unexported,由 Listener[H] 实现,断言即可获得具体实例化类型)。
type listenerAPI interface {
	init(impl any) error
	enabled() bool
	arm()
	protocol() string
	confType() reflect.Type
	setConf(v any)
	setLog(log *slog.Logger)
	serve(ctx context.Context, impl any) error
}

// listenerAt 从字段地址提取 Listener;
// 未导出字段的 Value 带 flagRO,须按 WithRef 的方式经 unsafe 指针重建。
func listenerAt(addr reflect.Value, t reflect.Type, name string) (listenerAPI, error) {
	p := reflect.NewAt(addr.Type().Elem(), addr.UnsafePointer()).Interface()
	l, ok := p.(listenerAPI)
	if !ok {
		return nil, errors.Errorf("WithConfig: 字段 %s.%s 不是 weaver.Listener", t, name)
	}
	return l, nil
}

// listeners 收集组件内全部 Listener 字段。
func listeners(impl any) ([]listenerAPI, error) {
	s := reflect.ValueOf(impl).Elem()
	t := s.Type()

	var out []listenerAPI
	for i := 0; i < t.NumField(); i++ {
		if !strings.HasPrefix(t.Field(i).Type.Name(), "Listener[") {
			continue
		}
		l, err := listenerAt(s.Field(i).Addr(), t, t.Field(i).Name)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// WithConfig 遍历组件字段完成装配:
//   - Listener 字段:以组件自身 init 出对应服务端,并按 conf tag 注入配置、注册热更新;
//   - WithConfig 字段:沿用既有 SetConfig 注入链路(HTTP/TCP/UDP 服务已收敛为
//     Listener,其 logger 与配置注入由 initListener 单独处理);
//
// conf 为 nil 时仍装配 Listener(跳过配置注入,listener 保持未就绪)。
func (w *widget) WithConfig(v reflect.Value, impl any, log *slog.Logger) error {
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		panic(errors.Errorf("invalid non pointer to struct value: %v", v))
	}

	s := v.Elem()
	t := s.Type()
	seen := map[string]bool{}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		if strings.HasPrefix(f.Type.Name(), "Listener[") {
			l, err := listenerAt(s.Field(i).Addr(), t, f.Name)
			if err != nil {
				return err
			}
			if err := w.initListener(l, impl, f, seen, log); err != nil {
				return err
			}
			continue
		}

		if !isConfigManagedType(f.Type.Name()) {
			continue
		}

		key := fieldConfTag(f)
		if key == "" {
			w.logger("weaver").Info("未找到配置依赖标签", "struct", t, "fieldName", f.Name, "fieldType", f.Type, "tag", f.Tag)
			continue
		}

		cfgField, ok := f.Type.FieldByName("config")
		if !ok {
			w.logger("weaver").Error("WithConfig 结构体缺少 config 字段", "fieldType", f.Type)
			continue
		}

		// 统一通过 SetConfig 注入:先解析到临时变量,再加锁写入,避免与读取方竞争
		fc := s.Field(i).Addr()
		inject := func() {
			tmp := reflect.New(cfgField.Type)
			if err := w.conf.UnmarshalKey(key, tmp.Interface()); err != nil {
				w.logger("weaver").Error("解析配置失败", "key", key, "err", err)
				return
			}
			fc.MethodByName("SetConfig").Call([]reflect.Value{tmp.Elem()})
		}

		if w.conf != nil {
			inject()
			w.WatchConfig(key, inject)
		}
	}
	return nil
}

// initListener 装配单个 Listener 字段:init 服务端、注入组件 logger、
// 注入配置并注册热更新。seen 用于拒绝同一协议被多个 Listener 字段绑定。
func (w *widget) initListener(l listenerAPI, impl any, f reflect.StructField, seen map[string]bool, log *slog.Logger) error {
	if err := l.init(impl); err != nil {
		return err
	}
	l.setLog(log)

	protocol := l.protocol()
	if seen[protocol] {
		return errors.Errorf("WithConfig: 字段 %s 重复绑定 %s 服务,同一协议只能声明一个 Listener", f.Name, protocol)
	}
	seen[protocol] = true

	key := fieldConfTag(f)
	if key == "" {
		w.logger("weaver").Info("Listener 字段未找到配置标签,跳过配置注入与自动启动",
			"impl", reflect.TypeOf(impl), "fieldName", f.Name, "tag", f.Tag)
		return nil
	}
	if w.conf == nil {
		w.logger("weaver").Info("未指定配置文件,Listener 跳过配置注入与自动启动", "fieldName", f.Name, "key", key)
		return nil
	}

	inject := func() {
		tmp := reflect.New(l.confType())
		if err := w.conf.UnmarshalKey(key, tmp.Interface()); err != nil {
			w.logger("weaver").Error("解析配置失败", "key", key, "err", err)
			return
		}
		l.setConf(tmp.Elem().Interface())
		l.arm()
	}

	inject()
	w.WatchConfig(key, inject)
	return nil
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

// WatchConfig 注册配置热更新回调,内部以独立锁保护,调用方无需持锁。
func (w *widget) WatchConfig(key string, fn func()) {
	w.watchMu.Lock()
	defer w.watchMu.Unlock()
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

// reloadConfig 重新执行配置注入回调使变更生效。
//
// 刻意不做 shutdown/start 重启:长驻阻塞型 Start 在该路径下会永久阻塞
// (startWG 永不释放),且重启无法重跑 Init;配置生效由注入回调完成。
func (w *widget) reloadConfig() {
	w.watchMu.Lock()
	fns := slices.Clone(w.watchConfig)
	w.watchMu.Unlock()

	for _, fn := range fns {
		fn()
	}
}

// start 并发启动所有组件的 Start 方法与已就绪 Listener 的自动 Serve。
// Start/Listener 允许长驻阻塞(如监听服务),因此 start 不等待其返回,
// 但会等待所有启动动作都已发起;同步快速失败时返回该错误,
// 其余失败或 panic 通过 cancel 上报,由调用方及其 app 监听 ctx 响应。
func (w *widget) start(ctx context.Context) error {
	w.mu.Lock()
	impls := make([]any, 0, len(w.components))
	for _, impl := range w.components {
		impls = append(impls, impl)
	}
	w.mu.Unlock()

	var (
		launched sync.WaitGroup
		errMu    sync.Mutex
		firstErr error
	)

	setErr := func(err error) {
		errMu.Lock()
		defer errMu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	for _, impl := range impls {
		_, hasStart := impl.(interface{ Start(_ context.Context) error })

		autos := make([]func(context.Context) error, 0, 3)
		ls, err := listeners(impl)
		if err != nil {
			w.log.Error("Component startup failed", "err", err)
			setErr(err)
			w.cancel()
			continue
		}
		for _, l := range ls {
			if !l.enabled() {
				continue
			}
			if hasStart {
				err := errors.Errorf("组件 %T 同时定义了 Start 与 Listener,只能二选一", impl)
				w.log.Error("Component startup failed", "err", err)
				setErr(err)
				w.cancel()
				continue
			}
			autos = append(autos, func(ctx context.Context) error {
				return l.serve(ctx, impl)
			})
		}

		if !hasStart && len(autos) == 0 {
			continue
		}

		starts := autos
		if hasStart {
			i, _ := impl.(interface{ Start(_ context.Context) error })
			starts = append(starts, i.Start)
		}

		for _, fn := range starts {
			launched.Add(1)
			w.startWG.Add(1)
			go func() {
				launched.Done()
				defer w.startWG.Done()
				defer func() {
					if e := recover(); e != nil {
						err := fmt.Errorf("component panic: %v", e)
						w.log.Error("Component startup panicked", "err", err)
						setErr(err)
						w.cancel()
					}
				}()

				if err := fn(ctx); err != nil {
					w.log.Error("Component startup failed", "err", err)
					setErr(err)
					w.cancel()
				}
			}()
		}
	}

	launched.Wait()
	errMu.Lock()
	defer errMu.Unlock()
	return firstErr
}

// shutdown 等待所有 Start 返回后,快照组件并在锁外依次调用 Shutdown,
// 避免用户实现死锁时永久占用 w.mu。
func (w *widget) shutdown(ctx context.Context) {
	w.startWG.Wait()

	type entry struct {
		name string
		impl any
	}

	w.mu.Lock()
	entries := make([]entry, 0, len(w.components))
	for c, impl := range w.components {
		entries = append(entries, entry{name: c, impl: impl})
	}
	w.mu.Unlock()

	for _, e := range entries {
		if i, ok := e.impl.(interface{ Shutdown(_ context.Context) error }); ok {
			if err := i.Shutdown(ctx); err != nil {
				w.log.Error("Component failed to shutdown", "component", e.name, "err", err)
			}
		}
	}
}
