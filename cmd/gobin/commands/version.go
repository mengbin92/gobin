package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// Build metadata. These variables are overridden by release builds via
// -ldflags -X.
var (
	Version   = "1.5.0"
	Commit    = ""
	BuildDate = ""
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
	if _, err := fmt.Fprintf(stdout, "Blog Static Site Generator %s\n", displayVersion(Version)); err != nil {
		return err
	}
	if Commit != "" {
		if _, err := fmt.Fprintf(stdout, "Commit: %s\n", Commit); err != nil {
			return err
		}
	}
	if BuildDate != "" {
		if _, err := fmt.Fprintf(stdout, "Build date: %s\n", BuildDate); err != nil {
			return err
		}
	}
	return nil
}

func displayVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "dev"
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}
