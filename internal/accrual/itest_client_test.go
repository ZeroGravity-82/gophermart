//go:build integration

package accrual

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestClient_GetAccrual_OK проверяет успешный ответ accrual-сервиса (HTTP 200).
func TestClient_GetAccrual_OK(t *testing.T) {
	// Arrange
	server := newAccrualTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		order, ok := isGetOrderRequest(r)
		require.True(t, ok)
		require.Equal(t, "123", order)
		writeJSON(t, w, http.StatusOK, GetAccrualAPIResponse{Order: order, Status: "PROCESSED", Accrual: floatPtr(1.5)})
	})
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	// Act
	resp, err := client.GetAccrual(ctx, "123")

	// Assert
	require.NoError(t, err)
	require.Equal(t, "123", resp.Order)
	require.Equal(t, "PROCESSED", resp.Status)
	require.NotNil(t, resp.Accrual)
	require.InEpsilon(t, 1.5, *resp.Accrual, 0.0001)
}

// TestClient_GetAccrual_204 проверяет, что код 204 преобразуется в ErrOrderNotProcessed.
func TestClient_GetAccrual_204(t *testing.T) {
	// Arrange
	server := newAccrualTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, ok := isGetOrderRequest(r)
		require.True(t, ok)
		w.WriteHeader(http.StatusNoContent)
	})
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	// Act
	_, err = client.GetAccrual(ctx, "999")

	// Assert
	require.ErrorIs(t, err, ErrOrderNotProcessed)
}

// TestClient_GetAccrual_RetryOn5xx проверяет, что при 5xx выполняются повторные запросы.
func TestClient_GetAccrual_RetryOn5xx(t *testing.T) {
	// Arrange
	calls := &atomicCounter{}
	server := newAccrualTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, ok := isGetOrderRequest(r)
		require.True(t, ok)

		n := calls.Inc()
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(t, w, http.StatusOK, GetAccrualAPIResponse{Order: "777", Status: "PROCESSING"})
	})
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	// Act
	_, err = client.GetAccrual(ctx, "777")

	// Assert
	require.NoError(t, err)
	require.GreaterOrEqual(t, calls.Load(), int64(3))
}

func floatPtr(v float64) *float64 { return &v }
