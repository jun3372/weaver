// Package make 实现 weaver make 命令:为组件接口生成实现结构体并完成代码生成注册。
package make

import (
	"fmt"
	"go/format"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"

	"github.com/jun3372/weaver/internal/generate"
)

var MakeCmd = &cobra.Command{
	Use:   "make <pkgdir> <InterfaceName>",
	Short: "为组件接口生成实现结构体",
	Long: `为组件接口生成实现结构体(内嵌 weaver.Implements[T])。

在接口所在包生成 <snake(接口名)>_impl.go,包含实现结构体与全部接口
方法的 stub,并自动执行代码生成完成组件注册。

用法:
  weaver make <pkgdir> <InterfaceName> [--force]

示例:
  weaver make ./greet Echo     # 生成 greet/echo_impl.go
`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		return runMake(args[0], args[1], force)
	},
}

func init() {
	MakeCmd.Flags().BoolP("force", "f", false, "目标文件已存在时覆盖")
}

func runMake(pkgdir, ifaceName string, force bool) error {
	pkgs, err := packages.Load(&packages.Config{
		Mode:  packages.NeedName | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:   pkgdir,
		Tests: false,
	}, ".")
	if err != nil {
		return errors.Wrapf(err, "加载包 %s 失败", pkgdir)
	}
	if len(pkgs) == 0 {
		return errors.Errorf("包 %s 为空", pkgdir)
	}
	pkg := pkgs[0]
	for _, e := range pkg.Errors {
		return errors.Errorf("包 %s 存在错误: %v", pkgdir, e)
	}

	obj := pkg.Types.Scope().Lookup(ifaceName)
	if obj == nil {
		return errors.Errorf("包 %s 中未找到类型 %q,候选: %s", pkgdir, ifaceName, strings.Join(scopeNames(pkg.Types.Scope()), ", "))
	}
	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return errors.Errorf("%q 不是接口", ifaceName)
	}

	out := filepath.Join(pkgdir, toSnake(ifaceName)+"_impl.go")
	if _, err := os.Stat(out); err == nil && !force {
		return errors.Errorf("%s 已存在,如需覆盖请加 --force", out)
	}

	src, err := renderImpl(pkg.Types, ifaceName, iface)
	if err != nil {
		return err
	}
	formatted, err := format.Source(src)
	if err != nil {
		return errors.Wrapf(err, "生成代码格式化失败:\n%s", src)
	}
	if err := os.WriteFile(out, formatted, 0o644); err != nil {
		return err
	}
	fmt.Println("生成", out)

	if err := generate.Generate(pkgdir, []string{"."}, generate.Options{BuildTags: "ignoreWeaverGen"}); err != nil {
		return errors.Wrap(err, "代码生成失败")
	}
	fmt.Println("代码生成完成")
	return nil
}

func scopeNames(scope *types.Scope) []string {
	var names []string
	for _, name := range scope.Names() {
		if _, ok := scope.Lookup(name).Type().Underlying().(*types.Interface); ok {
			names = append(names, name)
		}
	}
	return names
}
