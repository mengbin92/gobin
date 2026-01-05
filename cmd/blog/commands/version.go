package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// VersionCmd is the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  "Print the version number of the Gobin static site generator.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Blog Static Site Generator v%s\n", Version)
	},
}
