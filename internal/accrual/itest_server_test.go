//go:build integration

package accrual

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// newAccrualTestServer поднимает HTTP-заглушку accrual-сервиса для интеграционных тестов.
//
// Обработчик реализует ручку: GET /api/orders/{number}.
// Поведение задается через функцию handler.
func newAccrualTestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(handler))
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func isGetOrderRequest(r *http.Request) (orderNumber string, ok bool) {
	if r.Method != http.MethodGet {
		return "", false
	}
	if !strings.HasPrefix(r.URL.Path, "/api/orders/") {
		return "", false
	}
	return strings.TrimPrefix(r.URL.Path, "/api/orders/"), true
}

// atomicCounter is a tiny helper for counting calls.
type atomicCounter struct{ n atomic.Int64 }

func (c *atomicCounter) Inc() int64  { return c.n.Add(1) }
func (c *atomicCounter) Load() int64 { return c.n.Load() }
