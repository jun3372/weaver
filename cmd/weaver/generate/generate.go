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
	Short: "为 Weaver 组件生成注册代码(weaver_gen.go)",
	Long: `为 Weaver 组件生成注册代码(weaver_gen.go)。

用法:
  weaver generate [-tags taglist] [packages]

示例:
  weaver generate . ./greet`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			slog.Warn("Missing required argument")
			return
		}
		buildTags := "ignoreWeaverGen"
		var tags string
		cmd.Flags().StringVar(&tags, "tags", "", "Build tags to use when generating code")
		if tags != "" { // tags flag was specified=.
			buildTags = buildTags + "," + tags
		}

		if err := generate.Generate(".", args, generate.Options{BuildTags: buildTags}); err != nil {
			fmt.Println("Failed to generate code", err)
		}
	},
}
