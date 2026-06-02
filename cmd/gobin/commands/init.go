package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mengbin92/gobin/internal/log"
	"github.com/spf13/cobra"
)

// InitCmd is the init command
var InitCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new blog site",
	Long: `Initialize a new blog site with the default directory structure.

If name is not provided, uses current directory name.

	Example:
  gobin init my-blog
  gobin init`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var dir string
		if len(args) > 0 {
			dir = args[0]
		} else {
			dir, _ = os.Getwd()
		}

		return initializeSite(cmd.OutOrStdout(), dir)
	},
}

// initializeSite creates the directory structure and default files
func initializeSite(stdout io.Writer, dir string) error {
	// Get absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Error("failed to get absolute path", "error", err)
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get directory name for default title
	dirName := filepath.Base(absDir)

	fmt.Fprintf(stdout, "Initializing new blog site in: %s\n", absDir)
	log.Info("scaffolding new site", "dir", absDir)

	for _, d := range scaffoldDirectories() {
		path := filepath.Join(absDir, d)
		if err := os.MkdirAll(path, 0755); err != nil {
			log.Error("failed to create directory", "path", path, "error", err)
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	for path, content := range scaffoldFiles(dirName) {
		if err := writeFile(filepath.Join(absDir, path), content); err != nil {
			log.Error("failed to create scaffold file", "path", path, "error", err)
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
	}

	fmt.Fprintln(stdout, "Site initialized successfully!")
	fmt.Fprintln(stdout, "\nNext steps:")
	fmt.Fprintln(stdout, "  cd "+dirName)
	fmt.Fprintln(stdout, "  gobin build")
	fmt.Fprintln(stdout, "  gobin serve")

	return nil
}

func InitializeSite(stdout io.Writer, dir string) error {
	return initializeSite(stdout, dir)
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
