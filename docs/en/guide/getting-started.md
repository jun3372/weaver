# Getting Started

Weaver is a lightweight Go application framework focused on a simple, flexible and powerful component-based application building experience. It helps developers build modular, maintainable applications through dependency injection, configuration management and lifecycle management.

## Installation

Make sure your Go version is >= 1.27, then install the framework and the CLI tool:

```bash
# framework (as an application dependency)
go get github.com/jun3372/weaver

# CLI tool (init / generate / make / version)
go install github.com/jun3372/weaver/cmd/weaver@latest
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
        app.Logger(ctx).Info("App running")
        <-ctx.Done()
        return nil
    })
    if err != nil {
        panic(err)
    }
}
```

The second parameter of `weaver.Run` is the main-logic function, receiving the assembled Main component instance. If your app has no extra main logic and only needs to run components until an exit signal, use the one-liner `weaver.RunComponent(ctx, (*app)(nil))` (the second parameter exists only for type inference).

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

### 3. Add an HTTP server

Embed `weaver.Listener[weaver.Handler]` and register routes in `Init` — the framework injects the listen address, serves the handler automatically and manages graceful shutdown. **No `Start` needed**:

```go
type Api any

type api struct {
    weaver.Implements[Api]
    weaver.Listener[weaver.Handler] `conf:"http"`
}

func (a *api) Init(ctx context.Context) error {
    a.Mux().HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
        _, _ = w.Write([]byte("Hello, Weaver!"))
    })
    return nil
}
```

Add the listen address to `weaver.yaml`:

```yaml
http:
  addr: ":8080"
```

See [Listener: HTTP / TCP / UDP Server Hosting](/en/guide/listener) for the full picture — TCP/UDP servers, external frameworks (gin/echo) and the security options are covered on that page.

### 4. Generate component registration code

```bash
weaver generate .
```

This generates `weaver_gen.go` in the current package, wiring up component registration. Re-run it whenever a component interface changes; you can also hook it into `go generate` with `//go:generate weaver generate`. See [CLI Tools](/en/guide/generate).

### 5. Run the application

```bash
go run main.go -conf weaver.yaml
curl localhost:8080        # Hello, Weaver!
```

## Command-Line Flags

`weaver.Run` ships with two built-in flags:

| Flag | Environment Variable | Description |
| --- | --- | --- |
| `-conf` | `SERVICE_CONFIG` | Config file path; nothing is loaded when omitted |
| `-version` | `SERVICE_VERSION=true` | Print version info and exit |

Version info is an aligned key-value table (first line `Weaver <version>`, followed by Go / OS/Arch / Commit / Built; empty rows are omitted). It is also available via the `weaver version` subcommand:

```bash
$ weaver version
Weaver v0.1.6
  Go:      go1.27.1
  OS/Arch: darwin/arm64
  Commit:  f3dd60d
  Built:   2026-10-03 12:00:00 UTC
```

## CLI Commands

| Command | Description |
| --- | --- |
| `weaver init [dir]` | Scaffold a complete runnable project (HTTP server component, config, Makefile, Dockerfile and deploy manifests) |
| `weaver generate [packages]` | Generate component registration code `weaver_gen.go` |
| `weaver make <pkgdir> <Iface>` | Generate an implementation struct skeleton for a component interface |
| `weaver version` | Print version info |

## Next Steps

- Learn the [core concepts: components & dependency injection](/en/guide/concepts)
- Read [Listener: HTTP / TCP / UDP Server Hosting](/en/guide/listener)
- Read about [configuration](/en/guide/config) and [logging](/en/guide/logger)
- Browse the [examples](/en/examples)
