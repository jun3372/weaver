# Core Concepts: Components & Dependency Injection

The core of Weaver is an interface-based component system. Each component is defined by an **interface** and carried by an **implementation struct**. At startup, the framework builds the component graph from generated registration code and instantiates components on demand.

## Define a Component Interface

```go
package user

import "context"

type User interface {
    SayHello(ctx context.Context, name string) (string, error)
}
```

## Implement the Component

Embed `weaver.Implements[T]` to declare that the struct implements component interface T:

```go
package user

import (
    "context"
    "fmt"

    "github.com/jun3372/weaver"
)

type option struct {
    Source string
    Type   string
}

type userImpl struct {
    weaver.Implements[User]
    weaver.WithConfig[option] `conf:"user"`
}

func (u *userImpl) Init(ctx context.Context) error {
    u.Logger(ctx).Info("User component initialized")
    return nil
}

func (u *userImpl) SayHello(ctx context.Context, name string) (string, error) {
    return fmt.Sprintf("Hello, %s!", name), nil
}
```

## Inject Dependencies

Declare a `weaver.Ref[T]` field wherever another component is needed, and the framework injects the instance automatically:

```go
type app struct {
    weaver.Implements[weaver.Main]
    weaver.WithConfig[options] `conf:"app"`
    user weaver.Ref[user.User] // reference to the User component
}

func (a *app) Init(ctx context.Context) error {
    userComponent := a.user.Get()

    greeting, err := userComponent.SayHello(ctx, "World")
    if err != nil {
        return err
    }

    a.Logger(ctx).Info(greeting)
    return nil
}
```

`Ref` also works when embedded anonymously (e.g. `weaver.Ref[user.User]` as a struct field) — it is injected the same way.

## How It Works

- `weaver generate` scans packages for structs embedding `weaver.Implements[T]` and generates `weaver_gen.go` containing registration info for all components.
- `weaver.Run` builds three indexes from the registrations: by component name, by interface type, and by implementation type.
- Component instances are created via reflection in this order: cancel function → logger → config (`WithConfig`) → dependencies (`Ref`), followed by calling `Init`.
- Components are lazily instantiated: only referenced components are created.

## Common Errors

If you see an error like the one below, the registration code is missing or outdated:

```text
component github.com/xxx.User not found; maybe you forgot to run weaver generate
```

Re-run `weaver generate .` to fix it. See [Code Generation](/en/guide/generate).
