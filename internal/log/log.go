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
	AddSource bool   // include source location (only meaningful at debug)
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
