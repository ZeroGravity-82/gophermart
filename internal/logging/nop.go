package logging

import (
	"log/slog"
)

// NopLogger возращает логгер-заглушку.
func NopLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
