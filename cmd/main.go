package main

import (
	"github.com/spf13/cobra"

	"github.com/jun3372/weaver/cmd/generate"
	"github.com/jun3372/weaver/version"
)

var rootCmd = &cobra.Command{
	Use:   "weaver",
	Short: "Hugo is a very fast static site generator",
	Long: `A Fast and Flexible Static Site Generator built with
				  love by spf13 and friends in Go.
				  Complete documentation is available at http://hugo.spf13.com`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

func init() {
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(generate.GenerateCmd)
}

func main() {
	// packages.Load(&packages.Config{Mode: packages.NeedName})
	rootCmd.Execute()
}
