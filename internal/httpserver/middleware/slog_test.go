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
)

// TestSlogRequestLogger_LogsStatusAndRequestID проверяет логирование статуса ответа и request_id.
func TestSlogRequestLogger_LogsStatusAndRequestID(t *testing.T) {
	// Arrange
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	h := SlogRequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("nope"))
	}))

	r := httptest.NewRequest(http.MethodGet, "http://example.com/a?x=1", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.RequestIDKey, "rid-1"))
	w := httptest.NewRecorder()

	// Act
	h.ServeHTTP(w, r)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "status=418")
	assert.Contains(t, out, "request_id=rid-1")
	assert.Contains(t, out, "path=/a")
	assert.Contains(t, out, "query=\"x=1\"")
}

// TestSlogRequestLogger_OmitsEmptyOptionalFields проверяет, что пустые необязательные поля не логируются.
func TestSlogRequestLogger_OmitsEmptyOptionalFields(t *testing.T) {
	// Arrange
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	h := SlogRequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "http://example.com/a", nil)
	w := httptest.NewRecorder()

	// Act
	h.ServeHTTP(w, r)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "status=200")
	assert.NotContains(t, out, "request_id=")
	assert.NotContains(t, out, "query=")
}
