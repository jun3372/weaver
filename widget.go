package weaver

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/jun3372/weaver/internal/config"
	"github.com/jun3372/weaver/internal/defaults"
	"github.com/jun3372/weaver/runtime/codegen"
	"github.com/jun3372/weaver/runtime/logger"
)

type widget struct {
	ctx             context.Context
	conf            *viper.Viper
	envPrefix       string
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
	deps            map[string]map[string]bool             // 依赖边:deps[依赖者][被依赖者]=true,装配 Ref 时记录
	watchConfig     []func()
}

func newWidget(ctx context.Context, cancel context.CancelFunc, conf *viper.Viper, regs []*codegen.Registration, envPrefix string) *widget {
	w := widget{
		ctx:             ctx,
		conf:            conf,
		envPrefix:       envPrefix,
		cancel:          cancel,
		option:          new(config.Config),
		regsByName:      map[string]*codegen.Registration{},
		regsByInterface: map[reflect.Type]*codegen.Registration{},
		regsByImpl:      map[reflect.Type]*codegen.Registration{},
		components:      make(map[string]any),
		deps:            map[string]map[string]bool{},
		watchConfig:     []func(){},
	}

	for _, reg := range regs {
		w.regsByName[reg.Name] = reg
		w.regsByImpl[reg.Impl] = reg
		w.regsByInterface[reg.Interface] = reg
	}

	if w.conf != nil {
		// 注册环境变量绑定:系统配置(手工,剥掉 "weaver" 顶层避免重复)+ 组件配置(通用扫描)
		bindSystemConfig(w.conf, envPrefix)
		for _, reg := range regs {
			scanAndBind(w.conf, reg.Impl, envPrefix)
		}

		if err := conf.UnmarshalKey("weaver", &w.option); err != nil {
			slog.Warn("failed to unmarshal system config", "err", err)
		}
		// 环境变量覆盖:viper.UnmarshalKey 不尊重 env,需手动应用
		// 系统配置传空 topLevelKey,避免 env 名重复(WEAVER_LOGGER_LEVEL 而非 WEAVER_WEAVER_LOGGER_LEVEL)
		applyEnvVars(w.option, conf, envPrefix, "")
		// 系统配置 default tag 填充:仅对 viper 未设置的零值字段生效
		if err := defaults.Apply(w.option, "weaver", w.conf.IsSet); err != nil {
			slog.Warn("failed to apply system config defaults", "err", err)
		}
	}
	w.log = w.newLogger()
	if w.conf == nil {
		w.log.Info("未找到配置文件(-conf 参数与 SERVICE_CONFIG 环境变量均未指定,且当前目录及程序目录下无 weaver.*/config.*),组件配置将不会被注入")
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
		if i, ok := obj.(interface{ setExit(context.CancelFunc) }); ok {
			i.setExit(w.cancel)
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

	// WithRef:注入依赖并记录依赖边(供 shutdown 反向拓扑排序)
	if err := w.WithRef(obj, func(t reflect.Type) (any, error) {
		if dep, ok := w.regsByInterface[t]; ok && dep.Name != reg.Name {
			if w.deps[reg.Name] == nil {
				w.deps[reg.Name] = map[string]bool{}
			}
			w.deps[reg.Name][dep.Name] = true
		}
		return w.getInterface(t)
	}); err != nil {
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

// bindSystemConfig 为系统配置手工注册环境变量绑定,剥掉 "weaver" 顶层避免
// env 名重复(如 WEAVER_WEAVER_LOGGER_LEVEL)。
func bindSystemConfig(conf *viper.Viper, prefix string) {
	env := func(suffix string) string { return prefix + "_" + suffix }
	conf.BindEnv("weaver.logger.level", env("LOGGER_LEVEL"))
	conf.BindEnv("weaver.logger.type", env("LOGGER_TYPE"))
	conf.BindEnv("weaver.logger.addsource", env("LOGGER_ADDSOURCE"))
	conf.BindEnv("weaver.logger.component", env("LOGGER_COMPONENT"))
	conf.BindEnv("weaver.logger.file.filename", env("LOGGER_FILE_FILENAME"))
	conf.BindEnv("weaver.logger.file.maxsize", env("LOGGER_FILE_MAXSIZE"))
	conf.BindEnv("weaver.logger.file.maxage", env("LOGGER_FILE_MAXAGE"))
	conf.BindEnv("weaver.logger.file.maxbackups", env("LOGGER_FILE_MAXBACKUPS"))
	conf.BindEnv("weaver.logger.file.localtime", env("LOGGER_FILE_LOCALTIME"))
	conf.BindEnv("weaver.logger.file.compress", env("LOGGER_FILE_COMPRESS"))
}

// applyEnvVars 手动从环境变量读取值并覆盖 struct 字段。
// 用于解决 viper.UnmarshalKey 不尊重环境变量的限制。
// envPrefix 是环境变量前缀(如 "WEAVER"),topLevelKey 是配置顶层 key(如 "http")。
func applyEnvVars(v any, conf *viper.Viper, envPrefix string, topLevelKey string) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return
	}
	applyEnvToStruct(rv, conf, envPrefix, topLevelKey, "")
}

func applyEnvToStruct(s reflect.Value, conf *viper.Viper, envPrefix string, topLevelKey string, fieldPath string) {
	t := s.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		fv := s.Field(i)
		currentPath := fieldPath
		if currentPath == "" {
			currentPath = f.Name
		} else {
			currentPath = fieldPath + "." + f.Name
		}

		// 嵌套 struct
		if fv.Kind() == reflect.Struct {
			applyEnvToStruct(fv, conf, envPrefix, topLevelKey, currentPath)
			continue
		}
		if fv.Kind() == reflect.Pointer && fv.Type().Elem().Kind() == reflect.Struct && !fv.IsNil() {
			applyEnvToStruct(fv.Elem(), conf, envPrefix, topLevelKey, currentPath)
			continue
		}

		// 构造环境变量名
		// topLevelKey 为空时(系统配置),不包含顶层前缀,避免 WEAVER_WEAVER_LOGGER_LEVEL
		var envName string
		if topLevelKey == "" {
			envName = envPrefix + "_" + strings.ToUpper(strings.ReplaceAll(currentPath, ".", "_"))
		} else {
			envName = envPrefix + "_" + strings.ToUpper(topLevelKey) + "_" + strings.ToUpper(strings.ReplaceAll(currentPath, ".", "_"))
		}
		envVal := os.Getenv(envName)
		if envVal == "" {
			continue
		}

		// 解析并设置值
		setFieldFromString(fv, envVal)
	}
}

func setFieldFromString(fv reflect.Value, val string) {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)
	case reflect.Bool:
		if b, err := strconv.ParseBool(val); err == nil {
			fv.SetBool(b)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fv.Type() == reflect.TypeFor[time.Duration]() {
			if d, err := time.ParseDuration(val); err == nil {
				fv.SetInt(int64(d))
			}
		} else if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			fv.SetInt(n)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n, err := strconv.ParseUint(val, 10, 64); err == nil {
			fv.SetUint(n)
		}
	case reflect.Float32, reflect.Float64:
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			fv.SetFloat(n)
		}
	}
}

// listenerOptionType 从 Listener[H] 字段类型推断配置类型。
// 复用 init 的协议探测逻辑:单协议直接映射,聚合类型按组件实现决定。
func listenerOptionType(listenerType reflect.Type, implType reflect.Type) reflect.Type {
	// 从类型字符串提取类型参数 H
	// 例如:"weaver.Listener[http.Handler]" -> "http.Handler"
	typeStr := listenerType.String()
	start := strings.Index(typeStr, "[")
	end := strings.LastIndex(typeStr, "]")
	if start == -1 || end == -1 || start >= end {
		return nil
	}
	hTypeStr := typeStr[start+1 : end]

	// 根据类型参数字符串判断协议类型
	var hType reflect.Type
	switch {
	case strings.HasSuffix(hTypeStr, "http.Handler") || hTypeStr == "http.Handler":
		hType = httpHandlerType
	case strings.HasSuffix(hTypeStr, ".TCPHandler") || hTypeStr == "weaver.TCPHandler":
		hType = tcpHandlerType
	case strings.HasSuffix(hTypeStr, ".UDPPacketHandler") || hTypeStr == "weaver.UDPPacketHandler":
		hType = udpHandlerType
	case strings.HasSuffix(hTypeStr, ".Handler") || hTypeStr == "weaver.Handler":
		// 聚合类型:按组件实现决定
		found := handlerProtocols(implType)
		switch len(found) {
		case 0:
			return reflect.TypeFor[HTTPOption]()
		case 1:
			switch found[0] {
			case httpHandlerType:
				return reflect.TypeFor[HTTPOption]()
			case tcpHandlerType:
				return reflect.TypeFor[TCPOption]()
			case udpHandlerType:
				return reflect.TypeFor[UDPOption]()
			}
		}
		return nil
	default:
		return nil
	}

	// 单协议:直接映射
	switch hType {
	case httpHandlerType:
		return reflect.TypeFor[HTTPOption]()
	case tcpHandlerType:
		return reflect.TypeFor[TCPOption]()
	case udpHandlerType:
		return reflect.TypeFor[UDPOption]()
	}
	return nil
}

// scanAndBind 扫描组件实现类型的字段,为 WithConfig 与 Listener 字段的配置类型
// 注册环境变量绑定。
func scanAndBind(conf *viper.Viper, implType reflect.Type, envPrefix string) {
	// implType 是 struct 类型(非指针)
	if implType.Kind() == reflect.Pointer {
		implType = implType.Elem()
	}
	if implType.Kind() != reflect.Struct {
		return
	}

	for f := range implType.Fields() {
		fName := f.Type.Name()

		// WithConfig[T] 字段
		if isConfigManagedType(fName) {
			key := fieldConfTag(f)
			if key == "" {
				continue
			}
			cfgField, ok := f.Type.FieldByName("config")
			if !ok {
				continue
			}
			defaults.BindFields(conf, key, envPrefix, cfgField.Type)
			continue
		}

		// Listener[H] 字段
		if strings.HasPrefix(fName, "Listener[") {
			key := fieldConfTag(f)
			if key == "" {
				continue
			}
			if optType := listenerOptionType(f.Type, implType); optType != nil {
				defaults.BindFields(conf, key, envPrefix, optType)
			}
		}
	}
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
			applyEnvVars(tmp.Interface(), w.conf, w.envPrefix, key)
			if err := defaults.Apply(tmp.Interface(), key, w.conf.IsSet); err != nil {
				w.logger("weaver").Error("应用默认配置失败", "key", key, "err", err)
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
		applyEnvVars(tmp.Interface(), w.conf, w.envPrefix, key)
		if err := defaults.Apply(tmp.Interface(), key, w.conf.IsSet); err != nil {
			w.logger("weaver").Error("应用默认配置失败", "key", key, "err", err)
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

// shutdown 等待所有 Start 返回后,按依赖反向拓扑序依次调用 Shutdown:
// 被依赖的组件最后关闭(如 A 依赖 B,则先关 A 再关 B)。
// 无依赖关系的组件按名序关闭保证确定性;循环依赖的组件按名序兜底并告警。
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
	deps := make(map[string]map[string]bool, len(w.deps))
	for a, bs := range w.deps {
		deps[a] = maps.Clone(bs)
	}
	w.mu.Unlock()

	// dependedBy[b]=依赖 b 的组件集合;b 的依赖者全部关闭后 b 才可关闭
	remaining := make(map[string]any, len(entries))
	dependedBy := map[string]map[string]bool{}
	for _, e := range entries {
		remaining[e.name] = e.impl
	}
	for a, bs := range deps {
		for b := range bs {
			if _, ok := remaining[b]; !ok {
				continue
			}
			if dependedBy[b] == nil {
				dependedBy[b] = map[string]bool{}
			}
			dependedBy[b][a] = true
		}
	}

	for len(remaining) > 0 {
		var ready []string
		for name := range remaining {
			if len(dependedBy[name]) == 0 {
				ready = append(ready, name)
			}
		}
		if len(ready) == 0 {
			names := make([]string, 0, len(remaining))
			for name := range remaining {
				names = append(names, name)
			}
			slices.Sort(names)
			w.log.Warn("组件间存在循环依赖,关闭顺序无法完全按依赖排序,剩余组件按名序兜底关闭", "components", strings.Join(names, ", "))
			ready = names
		}
		slices.Sort(ready)

		for _, name := range ready {
			impl := remaining[name]
			delete(remaining, name)
			for dep := range deps[name] {
				delete(dependedBy[dep], name)
			}
			if i, ok := impl.(interface{ Shutdown(_ context.Context) error }); ok {
				if err := i.Shutdown(ctx); err != nil {
					w.log.Error("Component failed to shutdown", "component", name, "err", err)
				}
			}
		}
	}
}
