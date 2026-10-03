# Getting Started

Weaver is a lightweight Go application framework focused on a simple, flexible and powerful component-based application building experience. It helps developers build modular, maintainable applications through dependency injection, configuration management and lifecycle management.

## Installation

Make sure your Go version is >= 1.27, then run:

```bash
go get github.com/jun3372/weaver
```

## Create Your First App

### 1. Create the main application

```go
package main

import (
    "context"

    "github.com/jun3372/weaver"
)

type options struct {
    AppName string
    Version string
}

type app struct {
    weaver.Implements[weaver.Main]
    weaver.WithConfig[options] `conf:"app"`
}

func (a *app) Init(ctx context.Context) error {
    a.Logger(ctx).Info("App initialized", "name", a.Config().AppName)
    return nil
}

func main() {
    err := weaver.Run(context.Background(), func(ctx context.Context, app *app) error {
        // application logic
        app.Logger(ctx).Info("App running")
        <-ctx.Done()
        return nil
    })
    if err != nil {
        panic(err)
    }
}
```

### 2. Create a config file (weaver.yaml)

```yaml
app:
  appname: myapp
  version: 1.0.0

weaver:
  logger:
    level: info
    type: json
    file:
      filename: "./logs/weaver.log"
      maxsize: 100
      maxage: 7
      maxbackups: 10
      compress: true
```

### 3. Generate component registration code

```bash
# install the CLI tool first
go install github.com/jun3372/weaver/cmd/weaver@latest

# then use the local weaver command
weaver generate .
```

This generates `weaver_gen.go` in the current package, wiring up component registration. See [Code Generation](/en/guide/generate).

### 4. Run the application

```bash
go run main.go -conf weaver.yaml
```

## Command-Line Flags

`weaver.Run` ships with two built-in flags:

| Flag | Environment Variable | Description |
| --- | --- | --- |
| `-conf` | `SERVICE_CONFIG` | Config file path; nothing is loaded when omitted |
| `-version` | `SERVICE_VERSION=true` | Print version info and exit |

## Next Steps

- Learn the [core concepts: components & dependency injection](/en/guide/concepts)
- Read about [configuration](/en/guide/config) and [logging](/en/guide/logger)
- Browse the [examples](/en/examples)
