package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"

	"zerogravity-82/gophermart/internal/logging"
)

// TestRequestIDLogger_AddsRequestIDToContextLogger проверяет, что RequestIDLogger добавляет request_id в логгер,
// который хранится в контексте запроса.
func TestRequestIDLogger_AddsRequestIDToContextLogger(t *testing.T) {
	// Arrange
	buf := &bytes.Buffer{}
	baseLogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	h := RequestIDLogger()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Логгер должен быть уже обогащен request_id.
		logging.FromContext(r.Context()).Info("handler log")
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "http://example.com/a", nil)
	ctx := context.WithValue(r.Context(), middleware.RequestIDKey, "rid-1")
	ctx = logging.WithLogger(ctx, baseLogger)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()

	// Act
	h.ServeHTTP(w, r)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "handler log")
	assert.Contains(t, out, "request_id=rid-1")
}

// TestRequestIDLogger_DoesNotOverrideContextWhenNoRequestID проверяет, что при отсутствии request_id middleware
// не ломает логирование и не добавляет request_id.
func TestRequestIDLogger_DoesNotOverrideContextWhenNoRequestID(t *testing.T) {
	// Arrange
	buf := &bytes.Buffer{}
	baseLogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	h := RequestIDLogger()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.FromContext(r.Context()).Info("handler log")
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "http://example.com/a", nil)
	ctx := logging.WithLogger(r.Context(), baseLogger)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()

	// Act
	h.ServeHTTP(w, r)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "handler log")
	assert.NotContains(t, out, "request_id=")
}
