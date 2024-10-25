package codegen

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var globalRegistry registry

// globalRegistry is the global registry used by Register and Registered.
// Register registers a Service Weaver component.
func Register(reg Registration) {
	if err := globalRegistry.register(reg); err != nil {
		panic(err)
	}
}

// Registered returns the components registered with Register.
func Registered() []*Registration {
	return globalRegistry.allComponents()
}

// Find returns the registration of the named component.
func Find(name string) (*Registration, bool) {
	return globalRegistry.find(name)
}

// registry is a repository for registered Service Weaver components.
// Entries are typically added to the default registry by calls
// to Register in init functions in code generated by "weaver generate".
type registry struct {
	m          sync.Mutex
	components map[reflect.Type]*Registration // the set of registered components, by their interface types
	byName     map[string]*Registration       // map from full component name to registration
}

// Registration is the configuration needed to register a Service Weaver component.
type Registration struct {
	Name      string       // full package-prefixed component name
	Iface     reflect.Type // interface type for the component
	Impl      reflect.Type // implementation type (struct)
	Routed    bool         // True if calls to this component should be routed
	Listeners []string     // the names of any weaver.Listeners
}

func (r *registry) register(reg Registration) error {
	if err := verifyRegistration(reg); err != nil {
		return fmt.Errorf("Register(%q): %w", reg.Name, err)
	}

	r.m.Lock()
	defer r.m.Unlock()
	if r.components == nil {
		r.components = map[reflect.Type]*Registration{}
	}

	if r.byName == nil {
		r.byName = map[string]*Registration{}
	}

	ptr := &reg
	r.components[reg.Iface] = ptr
	r.byName[reg.Name] = ptr
	return nil
}

func verifyRegistration(reg Registration) error {
	if reg.Iface == nil {
		return errors.New("missing component type")
	}
	if reg.Iface.Kind() != reflect.Interface {
		return errors.New("component type is not an interface")
	}
	if reg.Impl == nil {
		return errors.New("missing implementation type")
	}
	if reg.Impl.Kind() != reflect.Struct {
		return errors.New("implementation type is not a struct")
	}
	return nil
}

func (r *registry) allComponents() []*Registration {
	r.m.Lock()
	defer r.m.Unlock()

	components := make([]*Registration, 0, len(r.components))
	for _, info := range r.components {
		components = append(components, info)
	}
	return components
}

func (r *registry) find(path string) (*Registration, bool) {
	r.m.Lock()
	defer r.m.Unlock()
	reg, ok := r.byName[path]
	return reg, ok
}