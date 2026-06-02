package log

import (
	"log/slog"
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
