package initialization

import (
	"os/exec"
	"text/template"
)

func execGo(dir string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var mainTmpl = template.Must(template.New("main").Parse(`package main

import (
	"context"

	"{{.Module}}/greet"

	"github.com/jun3372/weaver"
)

type option struct {
	Name string
}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.Ref[greet.T]
	weaver.WithConfig[option] ` + "`conf:\"app\"`" + `
}

func main() {
	if err := weaver.Run(context.Background(), func(ctx context.Context, a *app) error {
		a.Logger(ctx).Info("{{.Name}} 已启动", "conf", a.Config())
		resp, err := a.Ref.Get().Ping(ctx, "weaver")
		if err != nil {
			return err
		}
		a.Logger(ctx).Info("ping greet", "resp", resp)

		<-ctx.Done()
		return nil
	}); err != nil {
		panic(err)
	}
}
`))

var greetTmpl = template.Must(template.New("greet").Parse(`package greet

import (
	"context"

	"github.com/jun3372/weaver"
)

type option struct {
	Message string
}

type T interface {
	Ping(ctx context.Context, name string) (string, error)
}

type greet struct {
	weaver.Implements[T]
	weaver.WithConfig[option] ` + "`conf:\"greet\"`" + `
}

func (g *greet) Ping(ctx context.Context, name string) (string, error) {
	g.Logger(ctx).Info("ping", "name", name)
	return g.Config().Message + ", " + name, nil
}

func (g *greet) Init(ctx context.Context) error {
	g.Logger(ctx).Info("greet init", "conf", g.Config())
	return nil
}

func (g *greet) Shutdown(ctx context.Context) error {
	g.Logger(ctx).Info("greet shutdown")
	return nil
}
`))

var configTmpl = template.Must(template.New("config").Parse(`app:
  Name: "{{.Name}}"

greet:
  Message: "hello"

weaver:
  Logger:
    Level: "info"
    Type: "json"
`))
