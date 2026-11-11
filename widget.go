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
	"sync/atomic"
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
	reloadMu        sync.Mutex  // 保护 reloadTimer
	reloadTimer     *time.Timer // 热更新去抖定时器,nil 表示无待触发重载
	pendingReload   atomic.Bool // 有待处理的重载请求(去抖窗口内置位)
	execMu          sync.Mutex  // 串行化重载执行,同一时刻最多一轮注入回调在跑
	cancel          context.CancelFunc
	startWG         sync.WaitGroup                         // 等待所有组件 Start 返回
	regsByName      map[string]*codegen.Registration       // registrations by component name
	regsByInterface map[reflect.Type]*codegen.Registration // registrations by component interface type
	regsByImpl      map[reflect.Type]*codegen.Registration // registrations by component implementation type
	components      map[string]any                         // components, by name
	overrides       map[reflect.Type]any                   // 组件接口替换实例,weaver.Test 注入 fake 用
	deps            map[string]map[string]bool             // 依赖边:deps[依赖者][被依赖者]=true,装配 Ref 时记录
	watchConfig     []func()
	startErrOnce    sync.Once
	startErr        atomic.Pointer[error] // 启动阶段(Start/Listener serve)的首个错误,由 startErrOnce 保证只记一次
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
		overrides:       map[reflect.Type]any{},
		deps:            map[string]map[string]bool{},
		watchConfig:     []func(){},
	}

	for _, reg := range regs {
		w.regsByName[reg.Name] = reg
		w.regsByImpl[reg.Impl] = reg
		w.regsByInterface[reg.Interface] = reg
	}

	if w.conf != nil {
		// 系统配置注入:与组件配置共用唯一 env 解析路径 applyEnvVars,
		// 不再注册任何 BindEnv/AutomaticEnv 绑定。
		if err := conf.UnmarshalKey("weaver", &w.option); err != nil {
			slog.Warn("failed to unmarshal system config", "err", err)
		}
		// 环境变量覆盖:viper.UnmarshalKey 不尊重 env,由 applyEnvVars 统一应用。
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
			w.scheduleReload()
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
	// 测试替换优先:命中则直接返回 fake,真实实现不实例化
	if v, ok := w.overrides[t]; ok {
		return v, nil
	}

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

// warnUnrecognizedFields 装配期一次性诊断:框架靠反射类型名前缀
// ("Listener[" / "WithConfig[")识别服务字段与配置托管字段,类型名拿不到泛型实参
// 是语言限制,字符串识别是有意取舍;但自定义命名类型包装(type My weaver.Listener[H])
// 或外部同名类型(如 net.Listener)会被静默跳过 —— 组件看起来"没有服务"、
// "配置不生效"却没有任何线索。这里对名字疑似但前缀不匹配的字段输出 Warn。
//
// 注意:纯类型别名(type A = weaver.Listener[H])的反射名与原类型完全一致,
// 识别与功能均不受影响,不会触发本诊断;误报方向已由测试锁定。
// 同一组件同类字段合并为一条告警;由调用方(w.WithConfig)保证每组件装配期只跑一遍。
// 常见组件原语(Ref[...] / Implements[...])前缀优先排除,避免接口名恰好含
// "Listener"/"WithConfig" 字样时误报。
func warnUnrecognizedFields(log *slog.Logger, t reflect.Type) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}

	var listenerFields, configFields []string
	for f := range t.Fields() {
		name := f.Type.Name()
		switch {
		case strings.HasPrefix(name, "Listener["),
			strings.HasPrefix(name, "WithConfig["),
			strings.HasPrefix(name, "Ref["),
			strings.HasPrefix(name, "Implements["):
			continue // 框架可识别或属于其它组件原语
		case strings.Contains(name, "Listener"):
			listenerFields = append(listenerFields, f.Name)
		case strings.Contains(name, "WithConfig"):
			configFields = append(configFields, f.Name)
		}
	}

	if len(listenerFields) > 0 {
		log.Warn("字段类型疑似 weaver.Listener 的命名包装或外部同名类型,未被框架识别为服务字段,该字段不会托管任何服务;请直接内嵌 weaver.Listener[H],不要用自定义命名类型包装",
			"component", t, "fields", strings.Join(listenerFields, ", "))
	}
	if len(configFields) > 0 {
		log.Warn("字段类型疑似 weaver.WithConfig 的命名包装或外部同名类型,未被框架识别为配置托管字段,该字段的配置不会注入;请直接内嵌 weaver.WithConfig[T],不要用自定义命名类型包装",
			"component", t, "fields", strings.Join(configFields, ", "))
	}
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

	// 装配期一次性诊断:疑似 Listener/WithConfig 但未被识别的字段(命名类型包装、
	// 外部同名类型)输出告警,避免静默失效。每个组件仅在装配时走到这里一次。
	warnUnrecognizedFields(log, t)

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
			if !w.validateConfig(tmp.Elem(), key) {
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

// validateConfig 对实现了 Validate() error 的配置类型执行校验,
// 经指针断言,值接收者与指针接收者的 Validate 均生效;
// 失败时记录错误并返回 false,调用方保持旧配置不注入(热更新安全)。
func (w *widget) validateConfig(v reflect.Value, key string) bool {
	p, ok := v.Addr().Interface().(interface{ Validate() error })
	if !ok {
		return true
	}
	if err := p.Validate(); err != nil {
		w.logger("weaver").Error("配置校验失败,保留原有配置", "key", key, "err", err)
		return false
	}
	return true
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
		if !w.validateConfig(tmp.Elem(), key) {
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

// reloadConfigDebounce 是热更新事件去抖窗口:窗口内多次文件事件合并为
// 一次重载,避免编辑器多段写入/连续保存触发整批注入回调反复执行。
const reloadConfigDebounce = 100 * time.Millisecond

// scheduleReload 请求一次配置重载(固定窗口去抖):
//   - 去抖:窗口自首个事件起算,窗口内的后续事件只置位 pendingReload、
//     不重排定时器(固定窗口,规避 timer.Reset 与已触发 timer 的竞争),
//     到期一次执行,读到的必是最新配置;
//   - 异步:执行在定时器 goroutine 进行,fsnotify 回调立即返回,
//     不阻塞后续文件事件的投递;
//   - 序列化:重载执行经 execMu 串行,同一时刻最多一轮注入回调在跑,
//     执行期间的新事件会排下一轮,不会并发注入。
func (w *widget) scheduleReload() {
	w.reloadMu.Lock()
	defer w.reloadMu.Unlock()

	w.pendingReload.Store(true)
	if w.reloadTimer == nil {
		w.reloadTimer = time.AfterFunc(reloadConfigDebounce, w.runReload)
	}
}

// runReload 去抖窗口到期后执行一轮重载;并发触发下的多余 fire
// (pendingReload 已被清)直接跳过,不做无谓执行。
func (w *widget) runReload() {
	w.reloadMu.Lock()
	w.reloadTimer = nil
	w.reloadMu.Unlock()

	if !w.pendingReload.CompareAndSwap(true, false) {
		return
	}
	w.reloadConfig()
}

// reloadConfig 同步执行一轮配置注入回调:
//   - 经 execMu 串行,同一时刻最多一轮在跑(慢回调不并发重入);
//   - 单回调 panic 被 recover 并记日志,不影响其它组件的配置注入,
//     也不会打崩进程;
//   - 不持 watchMu 执行回调,热更新进行中注册新组件不受阻塞。
//
// 刻意不做 shutdown/start 重启:长驻阻塞型 Start 在该路径下会永久阻塞
// (startWG 永不释放),且重启无法重跑 Init;配置生效由注入回调完成。
// 文件事件路径应使用 scheduleReload(去抖);本函数供测试与显式刷新使用。
func (w *widget) reloadConfig() {
	w.execMu.Lock()
	defer w.execMu.Unlock()

	w.watchMu.Lock()
	fns := slices.Clone(w.watchConfig)
	w.watchMu.Unlock()

	for _, fn := range fns {
		w.runWatchFn(fn)
	}
}

// runWatchFn 隔离执行单个回调:panic 记 Error 日志后继续,不中断后续回调。
func (w *widget) runWatchFn(fn func()) {
	defer func() {
		if e := recover(); e != nil {
			w.log.Error("配置热更新回调 panic,已跳过该回调并继续执行其余回调", "panic", e)
		}
	}()
	fn()
}

// start 并发启动所有组件的 Start 方法与已就绪 Listener 的自动 Serve。
// Start/Listener 允许长驻阻塞(如监听服务),因此 start 只等待全部启动动作
// 都已发起即返回,不等待任何长驻 Start——否则 Run 永远进不了主逻辑。
//
// 错误上报契约:任一 Start/serve 在**关闭流程开始前**返回错误或 panic 时,
// 错误写入 startErr(首个错误胜出)并 cancel 应用 ctx;调用方在 cancel 之后、
// shutdown 完成之前经 startupError 读取,保证启动期错误必然反映在 Run 的
// 返回值上,不受 goroutine 调度时序影响。
//
// ctx 已取消后才返回的错误(包括长驻 Start 常见的
// `<-ctx.Done(); return ctx.Err()`)属于关闭的后果,不写入 startErr,
// 否则正常退出会被改写成启动失败;其中非取消类错误仍记日志便于诊断。start 自身的返回值仅覆盖同步即可见的
// 装配类失败(Listeners 扫描、Start/Listener 冲突),供测试断言。
func (w *widget) start(ctx context.Context) error {
	w.mu.Lock()
	impls := make([]any, 0, len(w.components))
	for _, impl := range w.components {
		impls = append(impls, impl)
	}
	w.mu.Unlock()

	var launched sync.WaitGroup

	for _, impl := range impls {
		_, hasStart := impl.(interface{ Start(_ context.Context) error })

		autos := make([]func(context.Context) error, 0, 3)
		ls, err := listeners(impl)
		if err != nil {
			w.log.Error("Component startup failed", "err", err)
			w.recordStartErr(err)
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
				w.recordStartErr(err)
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
						// 与返回错误同口径:关闭流程已开始时,panic 属于退出期间
						// 的异常,不应把正常退出改写成启动失败。
						if ctx.Err() != nil {
							return
						}
						w.recordStartErr(err)
						w.cancel()
					}
				}()

				if err := fn(ctx); err != nil {
					// ctx 已取消说明应用已进入关闭流程(信号 / Exit() / 其它组件
					// 先失败),此时的返回值是取消的后果而非启动失败。记录它会
					// 把正常退出覆盖成错误(如长驻 Start 常见写法
					// `<-ctx.Done(); return ctx.Err()` 会让 Run 返回
					// context canceled),因此不写入 startErr。
					// 取消类错误即正常退出路径,静默;其它错误仍记日志便于诊断。
					if ctx.Err() != nil {
						if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
							w.log.Error("Component exited with error during shutdown", "err", err)
						}
						return
					}
					w.log.Error("Component startup failed", "err", err)
					w.recordStartErr(err)
					w.cancel()
				}
			}()
		}
	}

	launched.Wait()
	return w.startupError()
}

// recordStartErr 记录启动阶段的首个错误;后续错误已被日志完整记录,
// 返回值统一取首个,避免启动失败原因被淹没。
func (w *widget) recordStartErr(err error) {
	w.startErrOnce.Do(func() { w.startErr.Store(&err) })
}

// startupError 返回启动阶段记录的首个错误,无则返回 nil。
//
// 读侧与写侧的同步由原子指针提供:写侧在 Start/serve goroutine 内
// 经 recordStartErr 写入并触发 cancel;读侧(Run / weaver.Test)在 ctx
// 取消、startWG.Wait 之后的 shutdown 中调用,原子存取保证可见性,
// 不存在数据竞争或半写入状态。
func (w *widget) startupError() error {
	if p := w.startErr.Load(); p != nil {
		return *p
	}
	return nil
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
