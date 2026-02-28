package accrual

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type readCloserWithErr struct{}

func (readCloserWithErr) Read(_ []byte) (int, error) { return 0, errors.New("read failure") }
func (readCloserWithErr) Close() error               { return nil }

// TestNew_CanValidatesBaseURL проверяет валидацию baseURL при создании клиента.
func TestNew_CanValidatesBaseURL(t *testing.T) {
	// Arrange
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{name: "empty", baseURL: " ", wantErr: true},
		{name: "invalid URL", baseURL: "http://[::1", wantErr: true},
		{name: "missing scheme", baseURL: "accrual.com", wantErr: true},
		{name: "ok", baseURL: "https://accrual.com", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			cl, err := New(tt.baseURL)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, cl)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cl)
			}
		})
	}
}

// TestGetAccrual_FailToGetAccrualWithEmptyOrderNumber проверяет ошибку при пустом номере заказа.
func TestGetAccrual_FailToGetAccrualWithEmptyOrderNumber(t *testing.T) {
	// Arrange
	cl, err := New("https://accrual.com")
	require.NoError(t, err)

	// Act
	_, err = cl.GetAccrual(context.Background(), "   ")

	// Assert
	require.Error(t, err)
}

// TestGetAccrual_CanGetAccrual проверяет успешное получение начисления из API.
func TestGetAccrual_CanGetAccrual(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		u, err := url.Parse(r.URL.String())
		require.NoError(t, err)
		assert.Equal(t, "accrual.com", u.Host)
		urlPath, err := url.JoinPath(urlGetAccrualPrefix, "12345678903")
		require.NoError(t, err)
		assert.Equal(t, urlPath, u.Path)

		assert.Equal(t, http.MethodGet, r.Method)

		respBody := `
{
  "order": "12345678903",
  "status": "PROCESSED",
  "accrual": 500
}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(respBody)),
			Header:     make(http.Header),
		}, nil
	})

	cl, err := New("https://accrual.com")
	require.NoError(t, err)
	cl.httpClient.client.HTTPClient.Transport = transport

	// Act
	res, err := cl.GetAccrual(context.Background(), "12345678903")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "12345678903", res.Order)
	assert.Equal(t, "PROCESSED", res.Status)
	assert.Equal(t, float64(500), *res.Accrual)
	assert.Equal(t, int32(1), calls.Load())
}

// TestGetAccrual_ReturnsErrOrderNotProcessedOn204 проверяет, что при ответе 204 клиент возвращает ErrOrderNotProcessed.
func TestGetAccrual_ReturnsErrOrderNotProcessedOn204(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(bytes.NewBuffer(nil)),
			Header:     make(http.Header),
		}, nil
	})

	cl, err := New("https://accrual.com")
	require.NoError(t, err)
	cl.httpClient.client.HTTPClient.Transport = transport

	// Act
	_, err = cl.GetAccrual(context.Background(), "12345678903")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOrderNotProcessed)
	assert.Equal(t, int32(1), calls.Load())
}

// TestGetAccrual_ReturnsErrTooManyRequestsOn429 проверяет, что при ответе 429 клиент возвращает ErrTooManyRequests.
func TestGetAccrual_ReturnsErrTooManyRequestsOn429(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		h := make(http.Header)
		h.Set("Retry-After", "2")
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewBufferString(`rate limited`)),
			Header:     h,
		}, nil
	})

	cl, err := New("https://accrual.com")
	require.NoError(t, err)
	cl.httpClient.client.HTTPClient.Transport = transport

	// Act
	_, err = cl.GetAccrual(context.Background(), "12345678903")

	// Assert
	var tmr ErrTooManyRequests
	require.Error(t, err)
	assert.ErrorAs(t, err, &tmr)
	assert.Equal(t, 2*time.Second, tmr.RetryAfter)
	assert.Equal(t, int32(1), calls.Load())
}
