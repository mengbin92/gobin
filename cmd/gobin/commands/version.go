package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// VersionCmd is the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  "Print the version number of the Gobin static site generator.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return printVersion(cmd.OutOrStdout())
	},
}

func printVersion(stdout io.Writer) error {
	_, err := fmt.Fprintf(stdout, "Blog Static Site Generator v%s\n", Version)
	return err
}
