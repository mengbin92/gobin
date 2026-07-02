package commands

import (
	"fmt"
	"os"

	"github.com/mengbin92/gobin/internal/config"
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

// InitLogging builds the global logger from priority resolution:
//   1. CLI flag (--verbose / --log-format / --log-file)
//   2. GOBIN_LOG_LEVEL / GOBIN_LOG_FORMAT / GOBIN_LOG_FILE env vars
//   3. config.yaml `logging` section
//   4. built-in default (info / text / stderr)
//
// The config argument is optional; when nil, only flag + env + default
// are considered. When the loaded config has a `logging` section with
// an unknown level / format / unwritable file, InitLogging returns an
// error so the user finds out at startup rather than at first log.
//
// Called from main.go's PersistentPreRun after LoadDefault; we accept
// the loaded config rather than re-parsing to keep the source of truth
// single.
func InitLogging(cfg *config.Config) error {
	values := loggingValuesFromConfig(cfg)
	values = overlayEnvOnValues(values)
	values = overlayFlagsOnValues(values)

	logger, err := log.NewFromValues(values)
	if err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	log.SetDefault(logger)
	return nil
}

// InitLoggingFromFlags is a v1.4.0-compatible entry point used by tests
// that do not load a config. Equivalent to InitLogging(nil) — flag +
// env + default, no project config layer.
func InitLoggingFromFlags() error {
	values := overlayEnvOnValues(log.ConfigValues{})
	values = overlayFlagsOnValues(values)
	logger, err := log.NewFromValues(values)
	if err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	log.SetDefault(logger)
	return nil
}

// loggingValuesFromConfig reads the logging section from cfg and
// returns the values to seed priority resolution with. Returns the
// zero value when cfg is nil or has no logging section.
func loggingValuesFromConfig(cfg *config.Config) log.ConfigValues {
	if cfg == nil || cfg.Logging == nil {
		return log.ConfigValues{}
	}
	return log.ConfigValues{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		File:   cfg.Logging.File,
	}
}

func overlayEnvOnValues(base log.ConfigValues) log.ConfigValues {
	merged := base
	if v := os.Getenv("GOBIN_LOG_LEVEL"); v != "" {
		merged.Level = v
	}
	if v := os.Getenv("GOBIN_LOG_FORMAT"); v != "" {
		merged.Format = v
	}
	if v := os.Getenv("GOBIN_LOG_FILE"); v != "" {
		merged.File = v
	}
	return merged
}

func overlayFlagsOnValues(base log.ConfigValues) log.ConfigValues {
	merged := base
	if verbose {
		merged.Level = "debug"
	}
	if logFormat != "" {
		merged.Format = logFormat
	}
	if logFile != "" {
		merged.File = logFile
	}
	return merged
}
