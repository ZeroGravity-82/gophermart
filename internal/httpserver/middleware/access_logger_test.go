package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"zerogravity-82/gophermart/internal/logging"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

// TestAccessLogger_LogsStatusAndRequestID проверяет логирование статуса ответа и request_id.
func TestAccessLogger_LogsStatusAndRequestID(t *testing.T) {
	// Arrange
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	h := middleware.RequestID(RequestIDLogger()(AccessLogger()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("nope"))
	}))))

	r := httptest.NewRequest(http.MethodGet, "http://example.com/a?x=1", nil)
	ctx := logging.WithLogger(r.Context(), logger)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	// Act
	h.ServeHTTP(w, r)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "status=418")
	assert.Contains(t, out, "request_id=")
	assert.Contains(t, out, "path=/a")
	assert.Contains(t, out, "query=\"x=1\"")
}

// TestAccessLogger_OmitsEmptyOptionalFields проверяет, что пустые необязательные поля не логируются.
func TestAccessLogger_OmitsEmptyOptionalFields(t *testing.T) {
	// Arrange
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	h := middleware.RequestID(RequestIDLogger()(AccessLogger()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))))

	r := httptest.NewRequest(http.MethodGet, "http://example.com/a", nil)
	ctx := logging.WithLogger(r.Context(), logger)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	// Act
	h.ServeHTTP(w, r)

	// Assert
	out := buf.String()
	assert.Contains(t, out, "status=200")
	// request_id добавляется в логгер через RequestIDLogger, поэтому он будет присутствовать.
	assert.Contains(t, out, "request_id=")
	assert.NotContains(t, out, "query=")
}
