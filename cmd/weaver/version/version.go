package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jun3372/weaver/version"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Hugo",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Hugo Static Site Generator %s -- HEAD\n", version.Version)
	},
}
