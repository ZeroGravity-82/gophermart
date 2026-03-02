// Package middleware содержит HTTP middleware (аутентификация, логирование и пр.).
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"zerogravity-82/gophermart/internal/auth"
)

type ctxKey string

const (
	userIDContextKey  ctxKey = "user_id"
	unauthorizedError string = "unauthorized"
)

var (
	// ErrUserIDNotFound возвращается, когда в контексте запроса нет userID.
	ErrUserIDNotFound = errors.New("user ID not found in the context")

	// ErrInvalidUserIDType возвращается, когда userID присутствует в контексте запроса, но имеет некорректный тип.
	ErrInvalidUserIDType = errors.New("user ID is not of valid type")
)

// UserIDFromContext извлекает идентификатор пользователя из контекста запроса.
func UserIDFromContext(ctx context.Context) (string, error) {
	value := ctx.Value(userIDContextKey)
	if value == nil {
		return "", ErrUserIDNotFound
	}
	v, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: expected string, got %T", ErrInvalidUserIDType, value)
	}
	return v, nil
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
