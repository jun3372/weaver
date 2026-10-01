// Package initialization 实现 weaver init 命令:初始化一个完整可运行的 Weaver 项目。
package initialization

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/jun3372/weaver/internal/generate"
)

var InitializationCmd = &cobra.Command{
	Use:   "init [dir]",
	Short: "初始化一个完整可运行的 Weaver 项目",
	Long: `初始化一个完整可运行的 Weaver 项目。

生成 main.go(Main 组件)、greet/ 示例组件、weaver.yaml 配置,
并自动执行 go mod 与代码生成。目标目录非空时需 --force。

用法:
  weaver init [dir] [--module <module path>]`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		return runInit(dir, cmd)
	},
}

func init() {
	InitializationCmd.Flags().String("module", "", "Go module 路径(默认取目录名)")
	InitializationCmd.Flags().BoolP("force", "f", false, "目标目录非空时强制初始化")
}

func runInit(dir string, cmd *cobra.Command) error {
	force, _ := cmd.Flags().GetBool("force")
	module, _ := cmd.Flags().GetString("module")

	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if module == "" {
		module = filepath.Base(abs)
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return err
		}
	} else if len(entries) > 0 && !force {
		return errors.Errorf("目录 %s 非空,如需覆盖请加 --force", abs)
	}

	project := project{Name: filepath.Base(abs), Module: module}
	files := map[string]*template.Template{
		"main.go":                          mainTmpl,
		"weaver.yaml":                      configTmpl,
		filepath.Join("greet", "greet.go"): greetTmpl,
	}
	for name, tmpl := range files {
		path := filepath.Join(abs, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		var sb strings.Builder
		if err := tmpl.Execute(&sb, project); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
			return err
		}
		fmt.Println("创建", path)
	}

	if err := goModInit(abs, module); err != nil {
		fmt.Printf("提示: 依赖初始化失败(%v),请手动执行:\n  cd %s && go mod init %s && go get github.com/jun3372/weaver@latest\n", err, abs, module)
	}

	if err := generate.Generate(abs, []string{"./..."}, generate.Options{BuildTags: "ignoreWeaverGen"}); err != nil {
		fmt.Printf("提示: 代码生成失败(%v),请稍后手动执行 weaver generate .\n", err)
	} else {
		fmt.Println("代码生成完成")
	}

	fmt.Printf("\n项目初始化完成:\n  cd %s && go run . -conf weaver.yaml\n", abs)
	return nil
}

func goModInit(abs, module string) error {
	if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
		return nil
	}
	if out, err := execGo(abs, "mod", "init", module); err != nil {
		return fmt.Errorf("go mod init: %v\n%s", err, out)
	}
	if out, err := execGo(abs, "get", "github.com/jun3372/weaver@latest"); err != nil {
		return fmt.Errorf("go get: %v\n%s", err, out)
	}
	return nil
}

type project struct {
	Name   string
	Module string
}
