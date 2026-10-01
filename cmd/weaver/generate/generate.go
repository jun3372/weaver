package generate

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/jun3372/weaver/internal/generate"
)

const (
	generatedCodeFile = "weaver_gen.go"
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
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			slog.Warn("Missing required argument")
			return
		}
		buildTags := "ignoreWeaverGen"
		if tags, _ := cmd.Flags().GetString("tags"); tags != "" {
			buildTags = buildTags + "," + tags
		}

		if err := generate.Generate(".", args, generate.Options{BuildTags: buildTags}); err != nil {
			fmt.Println("Failed to generate code", err)
		}
	},
}

func init() {
	GenerateCmd.Flags().String("tags", "", "生成代码时附加的构建标签,逗号分隔")
}
