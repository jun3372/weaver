// Package generate 实现 weaver generate 命令:为组件接口生成注册代码。
package generate

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/jun3372/weaver/internal/generate"
)

var GenerateCmd = &cobra.Command{
	Use:   "generate [packages]",
	Short: "为组件接口生成注册代码(weaver_gen.go)",
	Long: `扫描指定包中的 Weaver 组件接口,生成注册代码 weaver_gen.go。

生成文件带 //go:build !ignoreWeaverGen 构建标签,由其中的 init()
把组件注册到运行时注册表;组件接口变更后必须重新执行本命令。
packages 为 Go 包导入路径或相对目录,可一次传多个。

用法:
  weaver generate [-tags taglist] [packages]

示例:
  weaver generate .              # 生成当前包
  weaver generate . ./greet      # 一次生成多个包
  weaver generate -tags foo .    # 附加构建标签`,
	RunE: runGenerate,
}

func init() {
	GenerateCmd.Flags().String("tags", "", "生成代码时附加的构建标签,逗号分隔")
}

// runGenerate 是 generate 子命令的实现,返回非 nil 错误时由 cobra 写到
// stderr,main 以非 0 退出码结束。缺参数与生成失败都必须走错误路径,
// 否则 CI 无法感知生成失败(历史上这两种情况都静默返回 0)。
func runGenerate(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		// 根命令设置了 SilenceUsage,错误时 cobra 不再打印 usage,
		// 因此把用法附在错误信息里,保证用户能看到正确调用方式。
		return errors.Errorf("缺少必需参数:packages\n\n%s", cmd.UsageString())
	}
	buildTags := "ignoreWeaverGen"
	if tags, _ := cmd.Flags().GetString("tags"); tags != "" {
		buildTags = buildTags + "," + tags
	}

	if err := generate.Generate(".", args, generate.Options{BuildTags: buildTags}); err != nil {
		return errors.Wrap(err, "生成代码失败")
	}
	return nil
}
