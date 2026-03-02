package middleware

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"zerogravity-82/gophermart/internal/logging"
)

func RequestIDLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if requestID := middleware.GetReqID(ctx); requestID != "" {
				logger := logging.FromContext(ctx)
				logger = logger.With(slog.String("request_id", requestID))
				ctx = logging.WithLogger(ctx, logger)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}
