package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/auth"
)

// TestWithAuth_Fail проверяет, что middleware WithAuth возвращает 401 и единый текст в теле ответа при отсутствующем
// заголовке Authorization или при некорректном токена.
func TestWithAuth_Fail(t *testing.T) {
	// Arrange
	m, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{
			name:       "fail with missing authorization header",
			header:     "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "fail with invalid authorization header",
			header:     "Basic abc",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "fail with invalid token",
			header:     "Bearer not-a-token",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			h := WithAuth(m)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Authorization", tt.header)

			// Act
			h.ServeHTTP(w, r)

			// Assert
			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, "unauthorized\n", w.Body.String())
		})
	}
}

// TestWithAuth_OK проверяет, что middleware WithAuth добавляет userID в контекст и пропускает запрос дальше.
func TestWithAuth_OK(t *testing.T) {
	// Arrange
	m, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	tok, err := m.IssueAccessToken(context.Background(), "user-42")
	require.NoError(t, err)

	called := false
	h := WithAuth(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, err := UserIDFromContext(r.Context())
		require.NoError(t, err)
		assert.Equal(t, "user-42", userID)
		w.WriteHeader(http.StatusNoContent)
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	// Act
	h.ServeHTTP(w, r)

	// Assert
	assert.True(t, called)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestUserIDFromContext_WithNoUserID проверяет поведение UserIDFromContext при отсутствии значения в контексте.
func TestUserIDFromContext_WithNoUserID(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act
	_, err := UserIDFromContext(ctx)

	// Assert
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUserIDNotFound)
}

// TestUserIDFromContext_WithInvalidType проверяет поведение UserIDFromContext при некорректном типе значения в
// контексте.
func TestUserIDFromContext_WithInvalidType(t *testing.T) {
	// Arrange
	ctx := context.WithValue(context.Background(), userIDContextKey, 123)

	// Act
	_, err := UserIDFromContext(ctx)

	// Assert
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidUserIDType)
}
