# Code Generation

Weaver ships a CLI tool, `cmd/weaver`, that generates component registration code. We recommend installing it locally, then using the `weaver` command for everything.

## Installation

```bash
go install github.com/jun3372/weaver/cmd/weaver@latest
```

Verify the installation:

```bash
weaver version
```

If you prefer not to install, `go run` works as a one-off alternative (every `weaver` command below can be replaced with `go run github.com/jun3372/weaver/cmd/weaver`).

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

Applications also support `-version` (or the environment variable `SERVICE_VERSION=true`) to print version info:

```bash
go run main.go -version
```
