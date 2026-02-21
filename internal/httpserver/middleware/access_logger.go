package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"zerogravity-82/gophermart/internal/logging"
)

// AccessLogger пишет access-лог по завершении HTTP-запроса.
func AccessLogger() func(http.Handler) http.Handler {
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
			if r.RemoteAddr != "" {
				attrs = append(attrs, slog.String("remote_addr", r.RemoteAddr))
			}
			if r.URL.RawQuery != "" {
				attrs = append(attrs, slog.String("query", r.URL.RawQuery))
			}

			logger := logging.FromContext(r.Context())
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
