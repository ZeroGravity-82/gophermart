package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// SlogRequestLogger логирует запрос с помощью log/slog.
func SlogRequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int("size", ww.BytesWritten()),
				slog.Duration("duration", duration),
			}
			if reqID := middleware.GetReqID(r.Context()); reqID != "" {
				attrs = append(attrs, slog.String("request_id", reqID))
			}
			if r.RemoteAddr != "" {
				attrs = append(attrs, slog.String("remote_addr", r.RemoteAddr))
			}
			if r.URL.RawQuery != "" {
				attrs = append(attrs, slog.String("query", r.URL.RawQuery))
			}
			status := ww.Status()
			switch {
			case status >= 500:
				logger.Error("http request", attrs...)
			default:
				logger.Info("http request", attrs...)
			}
		})
	}
}
