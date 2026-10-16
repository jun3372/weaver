package weaver

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/jun3372/weaver/internal/reflection"
	"github.com/jun3372/weaver/runtime/codegen"
)

// weaver.Test 在测试进程内装配并启动全部组件,免去为验证组件逻辑而启动完整进程:
//
//	h := weaver.Test(t,
//		weaver.Override[Store](&fakeStore{}),            // 按组件接口替换为 fake
//		weaver.Conf("order", map[string]any{"timeout": "1s"}), // 注入组件配置
//	)
//	svc := h.Get[OrderService]()
//	_ = svc.Place(ctx, order)
//
// 组件从全局注册表(codegen.Registered,由 weaver generate 生成的 init 填充)解析,
// 自动 start 并在测试结束时(cancel + shutdown)优雅关闭。
//
// 限制:被 Override 的 fake 不注入 Ref 依赖,也不调用其 Init/Start/Shutdown。
// 带 Listener 字段的组件与生产行为一致:声明了 conf tag 即按配置(缺省为零值)
// 自动 Serve,测试中需以 Conf 提供监听地址或接受随机端口。

type testSettings struct {
	overrides map[reflect.Type]any
	conf      map[string]map[string]any // 配置键 -> 配置项,同 key 多次 Conf 按字段合并
}

// TestOption 定制 weaver.Test 的装配行为。
type TestOption func(*testSettings)

// Override 将组件接口 T 替换为给定实例,其真实实现不再实例化。
// fake 无需实现 Implements,也没有 logger/Exit 注入。
func Override[T any](impl T) TestOption {
	return func(s *testSettings) {
		s.overrides[reflection.Type[T]()] = any(impl)
	}
}

// Conf 注入组件配置,等价于配置文件中 key 键下的内容;
// 同 key 多次调用按字段合并,后写覆盖。
func Conf(key string, cfg map[string]any) TestOption {
	return func(s *testSettings) {
		if s.conf[key] == nil {
			s.conf[key] = map[string]any{}
		}
		maps.Copy(s.conf[key], cfg)
	}
}

// TestApp 是 weaver.Test 装配完成的测试应用,组件经 Get 获取。
type TestApp struct {
	t      testing.TB
	w      *widget
	cancel context.CancelFunc
}

// Test 装配并启动一个进程内测试应用,测试结束时自动优雅关闭。
func Test(t testing.TB, opts ...TestOption) *TestApp {
	settings := testSettings{
		overrides: map[reflect.Type]any{},
		conf:      map[string]map[string]any{},
	}
	for _, opt := range opts {
		opt(&settings)
	}

	conf := viper.New()
	for key, cfg := range settings.conf {
		conf.Set(key, cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w := newWidget(ctx, cancel, conf, codegen.Registered(), envPrefixName)
	// 覆盖表必须先于任何组件解析就位,否则被替换组件可能已实例化
	w.overrides = settings.overrides

	a := &TestApp{t: t, w: w, cancel: cancel}

	// 按注册名序预拉起全部组件:顺序确定,且把 Ref 依赖边记全供 shutdown 排序
	regs := slices.Clone(codegen.Registered())
	slices.SortFunc(regs, func(a, b *codegen.Registration) int { return strings.Compare(a.Name, b.Name) })
	for _, reg := range regs {
		if _, ok := settings.overrides[reg.Interface]; ok {
			continue
		}
		if _, err := w.getInterface(reg.Interface); err != nil {
			cancel()
			w.shutdown(context.Background())
			t.Fatalf("weaver.Test: 拉起组件 %s 失败: %v", reg.Name, err)
		}
	}

	if err := w.start(ctx); err != nil {
		cancel()
		w.shutdown(context.Background())
		t.Fatalf("weaver.Test: 启动失败: %v", err)
	}

	t.Cleanup(func() {
		// 必须先 cancel 再 shutdown:长驻 Start 阻塞在 ctx 上,
		// 直接 shutdown 会卡在 startWG.Wait
		a.cancel()
		a.w.shutdown(context.Background())
	})
	return a
}

// Get 返回组件接口 T 的实例,未注册或类型不符时使测试失败。
func (a *TestApp) Get[T any]() T {
	c, err := a.w.GetInterface(reflection.Type[T]())
	if err != nil {
		a.t.Fatalf("weaver.Test: 获取组件失败: %v", err)
	}
	v, ok := c.(T)
	if !ok {
		a.t.Fatalf("weaver.Test: 组件 %T 未实现 %T", c, *new(T))
	}
	return v
}
