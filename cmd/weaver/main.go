package main

import (
	"github.com/spf13/cobra"

	"github.com/jun3372/weaver/cmd/weaver/generate"
	"github.com/jun3372/weaver/cmd/weaver/initialization"
	"github.com/jun3372/weaver/cmd/weaver/version"
)

var rootCmd = &cobra.Command{
	Use:   "weaver",
	Short: "Hugo is a very fast static site generator",
}

func init() {
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(generate.GenerateCmd)
	rootCmd.AddCommand(initialization.InitializationCmd)
}

func main() {
	rootCmd.Execute()
}
