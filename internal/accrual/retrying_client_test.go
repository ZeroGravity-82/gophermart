package accrual

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDo_RetriesOn5xx проверяет, что клиент повторяет запросы при ответах 5xx.
func TestDo_RetriesOn5xx(t *testing.T) {
	// Arrange
	cl := newRetryingHTTPClient(
		2*time.Second,
		3,
		10*time.Millisecond,
		10*time.Millisecond,
		1500*time.Millisecond,
	)

	var calls atomic.Int32
	cl.client.HTTPClient.Transport = roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		if n < 3 {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString(`{"error":"boom"}`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`ok`)),
			Header:     make(http.Header),
		}, nil
	})

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://accrual.test/api/orders/1",
		nil,
	)
	require.NoError(t, err)

	// Act
	resp, err := cl.Do(req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), calls.Load())
}

// TestDo_DoesNotRetryOn4xx проверяет, что при ответах 4xx повторы не выполняются.
func TestDo_DoesNotRetryOn4xx(t *testing.T) {
	// Arrange
	cl := newRetryingHTTPClient(
		2*time.Second,
		3,
		10*time.Millisecond,
		10*time.Millisecond,
		1500*time.Millisecond,
	)

	var calls atomic.Int32
	cl.client.HTTPClient.Transport = roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewBufferString(`bad request`)),
			Header:     make(http.Header),
		}, nil
	})

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://accrual.test/api/orders/1",
		nil,
	)
	require.NoError(t, err)

	// Act
	resp, err := cl.Do(req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int32(1), calls.Load())
}

// TestDo_RetriesOn408 проверяет, что клиент повторяет запросы при ответах 408.
func TestDo_RetriesOn408(t *testing.T) {
	// Arrange
	cl := newRetryingHTTPClient(
		2*time.Second,
		3,
		10*time.Millisecond,
		10*time.Millisecond,
		1500*time.Millisecond,
	)

	var calls atomic.Int32
	cl.client.HTTPClient.Transport = roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		if n < 3 {
			return &http.Response{
				StatusCode: http.StatusRequestTimeout,
				Body:       io.NopCloser(bytes.NewBuffer(nil)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`ok`)),
			Header:     make(http.Header),
		}, nil
	})

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://accrual.test/api/orders/1",
		nil,
	)
	require.NoError(t, err)

	// Act
	resp, err := cl.Do(req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), calls.Load())
}

// TestDo_RetriesOn429WithRetryAfter проверяет, что клиент повторяет запросы при 429 и учитывает Retry-After.
func TestDo_RetriesOn429WithRetryAfter(t *testing.T) {
	// Arrange
	cl := newRetryingHTTPClient(
		5*time.Second,
		3,
		10*time.Millisecond,
		10*time.Millisecond,
		2*time.Second,
	)

	var calls atomic.Int32
	cl.client.HTTPClient.Transport = roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		if n == 1 {
			h := make(http.Header)
			h.Set("Retry-After", "1")
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(bytes.NewBufferString(`rate limited`)),
				Header:     h,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`ok`)),
			Header:     make(http.Header),
		}, nil
	})

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://accrual.test/api/orders/1",
		nil,
	)
	require.NoError(t, err)

	start := time.Now()

	// Act
	resp, err := cl.Do(req)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), calls.Load())
	assert.GreaterOrEqual(t, time.Since(start), 1*time.Second)
}

// TestDo_DoesNotRetryOnContextError проверяет, что при ошибках контекста (отмена, истечение) повторы не выполняются.
func TestDo_DoesNotRetryOnContextError(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		err  error
	}{
		{name: "context canceled", err: context.Canceled},
		{name: "context deadline exceeded", err: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := newRetryingHTTPClient(
				500*time.Millisecond,
				3,
				10*time.Millisecond,
				10*time.Millisecond,
				100*time.Millisecond,
			)

			var calls atomic.Int32
			cl.client.HTTPClient.Transport = roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				calls.Add(1)
				return nil, tt.err
			})

			req, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				"http://accrual.test/api/orders/1",
				nil,
			)
			require.NoError(t, err)

			// Act
			resp, err := cl.Do(req)
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}

			// Assert
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.err)
			assert.Equal(t, int32(1), calls.Load())
		})
	}
}
