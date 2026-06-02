package commands

import (
	"github.com/mengbin92/gobin/internal/log"
	"github.com/spf13/cobra"
)

// Global logging flags, shared across all commands.
var (
	verbose   bool
	logFormat string
	logFile   string
)

// AddGlobalFlags registers the persistent logging flags on the root command.
func AddGlobalFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "Log format: text or json")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Write logs to file (default: stderr)")
}

// InitLogging builds the global logger from the parsed flag values.
func InitLogging() {
	log.SetDefault(log.NewFromCLI(verbose, logFormat, logFile))
}
