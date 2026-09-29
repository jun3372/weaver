package codegen

import (
	"reflect"
	"slices"
	"testing"
)

type intfA interface{ A() }
type intfB interface{ B() }
type implA struct{}
type implB struct{}

func TestRegisterRejectsDuplicateNameWithDifferentImpl(t *testing.T) {
	r := &registry{}
	if err := r.register(Registration{
		Name:      "example.com/x.A",
		Interface: reflect.TypeFor[intfA](),
		Impl:      reflect.TypeFor[implA](),
	}); err != nil {
		t.Fatalf("首次注册不应报错: %v", err)
	}

	err := r.register(Registration{
		Name:      "example.com/x.A",
		Interface: reflect.TypeFor[intfA](),
		Impl:      reflect.TypeFor[implB](),
	})
	if err == nil {
		t.Fatal("同名不同实现的注册应报错，而不是静默覆盖")
	}
}

func TestRegisterRejectsDuplicateInterfaceWithDifferentName(t *testing.T) {
	r := &registry{}
	if err := r.register(Registration{
		Name:      "example.com/x.A",
		Interface: reflect.TypeFor[intfA](),
		Impl:      reflect.TypeFor[implA](),
	}); err != nil {
		t.Fatalf("首次注册不应报错: %v", err)
	}

	if err := r.register(Registration{
		Name:      "example.com/y.A",
		Interface: reflect.TypeFor[intfA](),
		Impl:      reflect.TypeFor[implB](),
	}); err == nil {
		t.Fatal("同接口不同名称的注册应报错")
	}
}

func TestRegisterIsIdempotentForIdenticalRegistration(t *testing.T) {
	r := &registry{}
	reg := Registration{
		Name:      "example.com/x.A",
		Interface: reflect.TypeFor[intfA](),
		Impl:      reflect.TypeFor[implA](),
	}
	if err := r.register(reg); err != nil {
		t.Fatalf("首次注册: %v", err)
	}
	if err := r.register(reg); err != nil {
		t.Fatalf("完全相同的重复注册应幂等: %v", err)
	}
}

func TestAllComponentsIsDeterministic(t *testing.T) {
	r := &registry{}
	regs := []Registration{
		{Name: "example.com/z.C", Interface: reflect.TypeFor[intfB](), Impl: reflect.TypeFor[implB]()},
		{Name: "example.com/a.B", Interface: reflect.TypeFor[intfA](), Impl: reflect.TypeFor[implA]()},
	}
	for _, reg := range regs {
		if err := r.register(reg); err != nil {
			t.Fatalf("register %s: %v", reg.Name, err)
		}
	}

	names := []string{}
	for _, c := range r.allComponents() {
		names = append(names, c.Name)
	}
	if !slices.IsSorted(names) {
		t.Fatalf("allComponents 应按名称排序保证确定性，实际: %v", names)
	}
}
