package main

import (
	"github.com/spf13/cobra"

	"github.com/jun3372/weaver/cmd/weaver/generate"
	"github.com/jun3372/weaver/cmd/weaver/initialization"
	"github.com/jun3372/weaver/cmd/weaver/make"
	"github.com/jun3372/weaver/cmd/weaver/version"
)

var rootCmd = &cobra.Command{
	Use:   "weaver",
	Short: "Weaver 组件化应用框架的代码生成与项目脚手架 CLI",
}

func init() {
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(generate.GenerateCmd)
	rootCmd.AddCommand(initialization.InitializationCmd)
	rootCmd.AddCommand(make.MakeCmd)
}

func main() {
	rootCmd.Execute()
}
