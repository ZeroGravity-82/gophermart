// Package middleware содержит HTTP middleware (аутентификация, логирование и пр.).
package middleware

import (
	"context"
	"net/http"
	"strings"

	"zerogravity-82/gophermart/internal/auth"
)

type ctxKey string

const (
	userIDContextKey  ctxKey = "user_id"
	unauthorizedError string = "unauthorized"
)

// UserIDFromContext извлекает идентификатор пользователя из контекста запроса.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDContextKey).(string)
	return v, ok
}

// WithAuth возвращает middleware, который проверяет access-токен и добавляет userID в контекст.
func WithAuth(m *auth.JWTManager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if h == "" {
				http.Error(w, unauthorizedError, http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(h, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, unauthorizedError, http.StatusUnauthorized)
				return
			}
			claims, err := m.ParseAccessToken(r.Context(), parts[1])
			if err != nil {
				http.Error(w, unauthorizedError, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDContextKey, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
