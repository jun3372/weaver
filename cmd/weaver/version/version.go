package version

import (
	"github.com/spf13/cobra"

	"github.com/jun3372/weaver/version"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "输出 Weaver 版本信息",
	Long: `输出 Weaver 版本信息。

依次显示 Version / Go Version / Git Commit / Build Time,
可通过 -ldflags 在构建时注入;未注入的项显示为 (dev) 或省略。
输出内容与 weaver -version 标志一致。`,
	Run: func(cmd *cobra.Command, args []string) {
		version.PrintVersion()
	},
}
