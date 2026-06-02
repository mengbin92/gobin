package log

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// FromContext extracts a logger from ctx, or returns the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return l
	}
	return defaultLogger
}

// IntoContext returns a copy of ctx carrying the given logger.
func IntoContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}
