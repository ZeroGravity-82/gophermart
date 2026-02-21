package logging

import (
	"context"
	"log/slog"
)

type ctxKey string

const loggerContextKey ctxKey = "logger"

// WithLogger сохраняет логгер в контекст.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey, logger)
}

// FromContext возвращает логгер из контекста. Если логгера в контексте нет, то возвращает логгер-заглушку.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return NopLogger()
	}
	if v, ok := ctx.Value(loggerContextKey).(*slog.Logger); ok && v != nil {
		return v
	}
	return NopLogger()
}
