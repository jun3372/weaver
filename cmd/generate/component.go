package generate

import (
	"go/types"
	"path"
	"sort"
)

type component struct {
	intf          *types.Named        // component interface
	impl          *types.Named        // component implementation
	router        *types.Named        // router, or nil if there is no router
	routingKey    types.Type          // routing key, or nil if there is no router
	routedMethods map[string]bool     // the set of methods with a routing function
	isMain        bool                // intf is weaver.Main
	refs          []*types.Named      // List of T where a weaver.Ref[T] field is in impl struct
	listeners     []string            // Names of listener fields declared in impl struct
	noretry       map[string]struct{} // Methods that should not be retried
}

func fullName(t *types.Named) string {
	return path.Join(t.Obj().Pkg().Path(), t.Obj().Name())
}

// intfName returns the component interface name.
func (c *component) intfName() string {
	return c.intf.Obj().Name()
}

// implName returns the component implementation name.
func (c *component) implName() string {
	return c.impl.Obj().Name()
}

// fullIntfName returns the full package-prefixed component interface name.
func (c *component) fullIntfName() string {
	return fullName(c.intf)
}

// methods returns the component interface's methods.
func (c *component) methods() []*types.Func {
	underlying := c.intf.Underlying().(*types.Interface)
	methods := make([]*types.Func, underlying.NumMethods())
	for i := 0; i < underlying.NumMethods(); i++ {
		methods[i] = underlying.Method(i)
	}

	// Sort the component's methods deterministically. This allows a developer
	// to re-order the interface methods without the generated code changing.
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name() < methods[j].Name()
	})
	return methods
}
