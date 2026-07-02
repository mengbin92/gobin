package log

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Level is a string log level that maps onto slog.Level.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Format selects the slog handler used for output.
type Format string

const (
	FormatText Format = "text" // terminal-friendly (default)
	FormatJSON Format = "json" // machine-readable (CI / pipelines)
)

// Options configures a logger built by New.
type Options struct {
	Level     Level  // default info
	Format    Format // default text
	Output    string // "stderr" (default), "stdout", or a file path
	AddSource bool   // AddSource includes source location; effective only at debug level.
}

// Default returns the baseline configuration: INFO, text, stderr.
func Default() Options {
	return Options{Level: LevelInfo, Format: FormatText, Output: "stderr"}
}

func parseLevel(l Level) slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	case LevelInfo:
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// New builds a *slog.Logger from opts.
func New(opts Options) *slog.Logger {
	level := parseLevel(opts.Level)
	handlerOpts := &slog.HandlerOptions{
		Level:     level,
		AddSource: opts.AddSource && level <= slog.LevelDebug,
	}
	writer := resolveWriter(opts.Output)

	var handler slog.Handler
	switch opts.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(writer, handlerOpts)
	default:
		handler = slog.NewTextHandler(writer, handlerOpts)
	}
	return slog.New(handler)
}

// NewFromCLI builds a logger from CLI flag values.
// verbose=true raises the level to debug and enables source locations.
func NewFromCLI(verbose bool, format, logFile string) *slog.Logger {
	opts := Default()
	if verbose {
		opts.Level = LevelDebug
		opts.AddSource = true
	}
	if format != "" {
		opts.Format = Format(format)
	}
	if logFile != "" {
		opts.Output = logFile
	}
	return New(opts)
}

// WithComponent returns a child logger tagged with the given component name.
func WithComponent(logger *slog.Logger, component string) *slog.Logger {
	return logger.With("component", component)
}

// ----- global default logger -----

// defaultLogger is set once at startup (PersistentPreRun) and is not safe for concurrent mutation.
var defaultLogger *slog.Logger

func init() {
	defaultLogger = New(Default())
}

// SetDefault installs l as the package default and the slog default.
func SetDefault(l *slog.Logger) {
	defaultLogger = l
	slog.SetDefault(l)
}

// GetDefault returns the current package default logger.
func GetDefault() *slog.Logger { return defaultLogger }

// Convenience helpers forwarding to the default logger.
func Debug(msg string, args ...any) { defaultLogger.Debug(msg, args...) }
func Info(msg string, args ...any)  { defaultLogger.Info(msg, args...) }
func Warn(msg string, args ...any)  { defaultLogger.Warn(msg, args...) }
func Error(msg string, args ...any) { defaultLogger.Error(msg, args...) }

// ConfigValues is the resolved logging configuration. It is a leaf type
// (no struct dependency on the config package) so the log package can
// accept values from any source without creating an import cycle.
//
// The fields mirror config.LoggingConfig. Callers in higher layers read
// the three fields from the loaded config and pass them here.
type ConfigValues struct {
	Level  string
	Format string
	File   string
}

// NewFromValues builds a logger from a ConfigValues. Empty fields are
// treated as "use the default" so callers can pass partially-populated
// values from priority resolution.
//
// Unlike NewFromCLI, unknown level / format values produce an error
// rather than silently falling back. This matches the v1.5.0 spec:
// typos in committed config.yaml are project-level errors, while
// transient CLI flag typos stay tolerant.
func NewFromValues(values ConfigValues) (*slog.Logger, error) {
	opts := Default()
	if values.Level != "" {
		if _, err := parseLevelStrict(Level(values.Level)); err != nil {
			return nil, err
		}
		opts.Level = Level(values.Level)
	}
	if values.Format != "" {
		if err := validateFormat(Format(values.Format)); err != nil {
			return nil, err
		}
		opts.Format = Format(values.Format)
	}
	if values.File != "" {
		if err := validateFilePath(values.File); err != nil {
			return nil, err
		}
		opts.Output = values.File
	}
	return New(opts), nil
}

// NewFromEnv reads GOBIN_LOG_LEVEL, GOBIN_LOG_FORMAT, GOBIN_LOG_FILE
// from the environment and applies them as overrides on top of base.
// Empty env vars are ignored; empty base fields are ignored too.
//
// This is the v1.5.0 entry point used when the user wants env-driven
// config (e.g. docker compose) without changing config.yaml.
func NewFromEnv(base ConfigValues) (*slog.Logger, error) {
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
	return NewFromValues(merged)
}

func parseLevelStrict(l Level) (slog.Level, error) {
	switch l {
	case LevelDebug, LevelInfo, LevelWarn, LevelError:
		return parseLevel(l), nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid logging level %q (want debug|info|warn|error)", string(l))
	}
}

func validateFormat(f Format) error {
	switch f {
	case FormatText, FormatJSON:
		return nil
	default:
		return fmt.Errorf("invalid logging format %q (want text|json)", string(f))
	}
}

func validateFilePath(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	if info, err := os.Stat(dir); err != nil {
		return fmt.Errorf("log file parent directory %q: %w", dir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("log file parent %q is not a directory", dir)
	}
	// Test writability with a create-and-close. This catches read-only
	// mounts and permission issues early instead of failing at first log.
	if err := probeWritable(path); err != nil {
		return fmt.Errorf("log file %q not writable: %w", path, err)
	}
	return nil
}

func probeWritable(path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}
