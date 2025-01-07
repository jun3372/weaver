package initialization

import (
	"github.com/spf13/cobra"
)

var InitializationCmd = &cobra.Command{
	Use:   "init",
	Short: "Print the version number of Hugo",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
	},
}
