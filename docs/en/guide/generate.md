# CLI Tools

Weaver ships a CLI tool, `cmd/weaver`, covering project scaffolding, component implementation generation and registration codegen. We recommend installing it locally, then using the `weaver` command for everything.

## Installation

```bash
go install github.com/jun3372/weaver/cmd/weaver@latest
```

Verify the installation:

```bash
weaver version
```

If you prefer not to install, `go run` works as a one-off alternative (every `weaver` command below can be replaced with `go run github.com/jun3372/weaver/cmd/weaver`).

## Scaffold a Project

`weaver init` scaffolds a complete, runnable Weaver project:

```bash
weaver init myapp --module example.com/myapp
cd myapp
go run ./cmd -conf etc/weaver.yaml
curl localhost:8080   # Hello, Weaver!
```

What you get:

- `cmd/main.go`: the Main component (declares dependencies and config, assembled and started by `weaver.RunComponent`)
- `internal/app/http.go`: an HTTP server component demonstrating `weaver.Listener[H]` + `Handler()` wiring (with a commented gin example) — no `Start` needed
- `etc/weaver.yaml`: config file (app / http server / weaver logger)
- `Makefile`: common targets — run / build / generate / test / docker-build / compose-up / k8s-apply
- `cmd/Dockerfile`: multi-stage build (next to main.go; golang builder + alpine runtime, `make docker-build`)
- `deploy/docker-compose.yaml` and `deploy/k8s.yaml`: ready-to-use orchestration files (Deployment + Service with health probes)
- Automatically runs `go mod init`, `go get github.com/jun3372/weaver@latest` and `weaver generate` (covering all subpackages)

Non-empty directories are rejected unless `--force` is given; `--module` defaults to the directory name.

## Generate a Component Implementation

After writing a component interface, `weaver make` generates the implementation skeleton and registers it automatically:

```bash
weaver make ./greet Echo
```

This creates `echo_impl.go` in the interface's package:

```go
// echo 是 Echo 的组件实现骨架,由 weaver make 生成。
type echo struct {
	weaver.Implements[Echo]
}

func (i *echo) Shout(p0 context.Context, p1 string) (string, error) {
	// TODO: implement
	var (
		r0 string
		r1 error
	)
	return r0, r1
}
```

- Unnamed parameters become `p0`, `p1`; results are returned as zero-value variables — just fill in the `// TODO`
- Method signatures must follow the component convention: first parameter `context.Context`, last result `error` (empty interfaces like `type T interface{}` work too)
- `weaver generate` runs automatically on the package afterwards
- Existing target files are never overwritten without `--force`; unknown interface names list the package's candidates

## Generate Registration Code

```bash
# generate for the current package
weaver generate .

# specific packages
weaver generate ./user ./chat
```

The tool scans target packages for structs embedding `weaver.Implements[T]` and generates `weaver_gen.go` in each package, containing the registration info that maps component interfaces to implementations. `weaver.Run` depends on these registrations to build the component graph.

The generated file carries the `ignoreWeaverGen` build tag so the tool never re-scans its own output.

## With go:generate

Add a `//go:generate` directive to use `go generate` directly:

```go
//go:generate weaver generate
package main
```

## Version

```bash
# CLI tool version
weaver version
```

Since v0.1.3 version info falls back automatically, so the output is meaningful even without any build-time injection:

- **Version line** (`Weaver x.y.z`): `-ldflags` injected value → module version when installed via `go install` → `(dev)`
- **Go**: the actual toolchain version
- **OS/Arch**: the target platform
- **Commit / Built**: VCS stamping embedded at build time (add `-buildvcs=true` when building; uncommitted changes append `-modified`, and a full 40-char hash is shortened to 7 chars)

Applications also support `-version` (or the environment variable `SERVICE_VERSION=true`) to print version info, with the same fallback logic:

```bash
go run main.go -version
```
