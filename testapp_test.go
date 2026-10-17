package weaver

import (
	"context"
	"reflect"
	"testing"

	"github.com/jun3372/weaver/internal/reflection"
	"github.com/jun3372/weaver/runtime/codegen"
)

// --- weaver.Test 测试组件(经 init 注册进全局注册表) ---

type greetIntf interface {
	Greet() string
}

var greetInited bool

type greetComp struct {
	Implements[greetIntf]
	WithConfig[struct{ Word string }] `conf:"greet"`
}

func (c *greetComp) Init(context.Context) error {
	greetInited = true
	return nil
}

func (c *greetComp) Greet() string { return c.Config().Word }

type fakeGreet struct{ word string }

func (f *fakeGreet) Greet() string { return f.word }

type farewellIntf interface {
	Farewell() string
}

var farewellShutdown bool

type farewellComp struct {
	Implements[farewellIntf]
}

func (c *farewellComp) Shutdown(context.Context) error {
	farewellShutdown = true
	return nil
}

func (c *farewellComp) Farewell() string { return "bye" }

func init() {
	codegen.Register(codegen.Registration{
		Name:      "testapp/greet",
		Interface: reflection.Type[greetIntf](),
		Impl:      reflect.TypeFor[greetComp](),
	})
	codegen.Register(codegen.Registration{
		Name:      "testapp/farewell",
		Interface: reflection.Type[farewellIntf](),
		Impl:      reflect.TypeFor[farewellComp](),
	})
}

func TestOverrideReplacesRealComponent(t *testing.T) {
	greetInited = false

	h := Test(t, Override[greetIntf](&fakeGreet{word: "hi"}))

	fake, ok := any(h.Get[greetIntf]()).(*fakeGreet)
	if !ok {
		t.Fatalf("Override 后应返回 fake 实例, got %T", h.Get[greetIntf]())
	}
	if fake.Greet() != "hi" {
		t.Fatalf("fake 未生效: got %q, want %q", fake.Greet(), "hi")
	}
	if greetInited {
		t.Fatal("被 Override 的组件真实实现不应被实例化")
	}
}

func TestConfInjectsConfig(t *testing.T) {
	h := Test(t, Conf("greet", map[string]any{"word": "hello"}))

	if got := h.Get[greetIntf]().Greet(); got != "hello" {
		t.Fatalf("Conf 未注入: got %q, want %q", got, "hello")
	}
}

func TestTestAppCleanupShutsDownComponents(t *testing.T) {
	farewellShutdown = false

	t.Run("inner", func(t *testing.T) {
		Test(t)
	})

	if !farewellShutdown {
		t.Fatal("测试结束后应自动 shutdown 全部组件")
	}
}
