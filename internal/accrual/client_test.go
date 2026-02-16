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
	cl.httpClient = &http.Client{Transport: transport}

	// Act
	res, err := cl.GetAccrual(context.Background(), "12345678903")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "12345678903", res.Order)
	assert.Equal(t, "PROCESSED", res.Status)
	assert.Equal(t, float64(500), *res.Accrual)
	assert.Equal(t, int32(1), calls.Load())
}

// TestGetAccrual_CanHandleNoContent204 проверяет обработку ответа 204 No Content.
func TestGetAccrual_CanHandleNoContent204(t *testing.T) {
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
	cl.httpClient = &http.Client{Transport: transport}

	// Act
	_, err = cl.GetAccrual(context.Background(), "12345678903")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOrderNotProcessed)
	assert.Equal(t, int32(1), calls.Load())
}

// TestGetAccrual_CanNotRetryOn4xxExcept408And429 проверяет, что на 4xx ошибки (кроме 408/429) повторы не делаются.
func TestGetAccrual_CanNotRetryOn4xxExcept408And429(t *testing.T) {
	// Arrange
	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "400", statusCode: http.StatusBadRequest},
		{name: "405", statusCode: http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				calls.Add(1)
				respBody := `{"order":"12345678903","status":"INVALID"}`
				return &http.Response{
					StatusCode: tt.statusCode,
					Body:       io.NopCloser(bytes.NewBufferString(respBody)),
					Header:     make(http.Header),
				}, nil
			})

			cl, err := New("https://accrual.com")
			require.NoError(t, err)
			cl.httpClient = &http.Client{Transport: transport}

			// Act
			_, err = cl.GetAccrual(context.Background(), "12345678903")

			// Assert
			require.Error(t, err)
			assert.Equal(t, int32(1), calls.Load())
		})
	}
}

// TestGetAccrual_CanRetryOn5xxAnd408 проверяет повторы запросов при 5xx и 408.
func TestGetAccrual_CanRetryOn5xxAnd408(t *testing.T) {
	// Arrange
	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "408", statusCode: http.StatusRequestTimeout},
		{name: "500", statusCode: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				n := calls.Add(1)
				if n < 3 {
					respBody := `{"order":"12345678903","status":"PROCESSING"}`
					return &http.Response{
						StatusCode: tt.statusCode,
						Body:       io.NopCloser(bytes.NewBufferString(respBody)),
						Header:     make(http.Header),
					}, nil
				}
				respBody := `{"order":"12345678903","status":"PROCESSED","accrual":10}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(respBody)),
					Header:     make(http.Header),
				}, nil
			})

			cl, err := New("https://accrual.com")
			require.NoError(t, err)
			cl.httpClient = &http.Client{Transport: transport}

			// Act
			res, err := cl.GetAccrual(context.Background(), "12345678903")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, "12345678903", res.Order)
			assert.Equal(t, "PROCESSED", res.Status)
			assert.Equal(t, float64(10), *res.Accrual)
			assert.Equal(t, int32(3), calls.Load())
		})
	}
}

// TestGetAccrual_CanRetryOn429 проверяет повторы запросов при 429 Too Many Requests.
func TestGetAccrual_CanRetryOn429(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		if n == 1 {
			h := make(http.Header)
			h.Set("Retry-After", "1")
			respBody := `{"order":"12345678903","status":"PROCESSING"}`
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(bytes.NewBufferString(respBody)),
				Header:     h,
			}, nil
		}
		respBody := `{"order":"12345678903","status":"PROCESSED","accrual":5}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(respBody)),
			Header:     make(http.Header),
		}, nil
	})

	cl, err := New("https://accrual.com")
	require.NoError(t, err)
	cl.httpClient = &http.Client{Transport: transport}

	start := time.Now()

	// Act
	res, err := cl.GetAccrual(context.Background(), "12345678903")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "12345678903", res.Order)
	assert.Equal(t, float64(5), *res.Accrual)
	assert.Equal(t, int32(2), calls.Load())
	assert.GreaterOrEqual(t, time.Since(start), 1*time.Second)
}

// TestGetAccrual_CanRetryOnReadBodyError проверяет повторы запросов при ошибке чтения тела ответа.
func TestGetAccrual_CanRetryOnReadBodyError(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: readCloserWithErr{}, Header: make(http.Header)}, nil
	})

	cl, err := New("https://accrual.com")
	require.NoError(t, err)
	cl.httpClient = &http.Client{Transport: transport}

	// Act
	_, err = cl.GetAccrual(context.Background(), "12345678903")

	// Assert
	require.Error(t, err)
	assert.Equal(t, int32(maxRetries+1), calls.Load())
}

// TestGetAccrual_CanRetryOnNonJSONBodyOnlyFor5xx проверяет повторы при не-JSON ответе только для 5xx.
func TestGetAccrual_CanRetryOnNonJSONBodyOnlyFor5xx(t *testing.T) {
	// Arrange
	tests := []struct {
		name        string
		statusCode  int
		wantRetries int32
	}{
		{name: "500 non-json retries", statusCode: http.StatusInternalServerError, wantRetries: int32(maxRetries + 1)},
		{name: "400 non-json no retry", statusCode: http.StatusBadRequest, wantRetries: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{
					StatusCode: tt.statusCode,
					Body:       io.NopCloser(bytes.NewBufferString("this is not json")),
					Header:     make(http.Header),
				}, nil
			})

			cl, err := New("https://accrual.com")
			require.NoError(t, err)
			cl.httpClient = &http.Client{Transport: transport}

			// Act
			_, err = cl.GetAccrual(context.Background(), "12345678903")

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.wantRetries, calls.Load())
		})
	}
}

// TestGetAccrual_DoesNotRetryOnContextError проверяет, что при ошибках контекста повторы не выполняются.
func TestGetAccrual_DoesNotRetryOnContextError(t *testing.T) {
	// Arrange
	tests := []struct {
		name      string
		err       error
		wantError error
	}{
		{name: "context cancelled", err: context.Canceled, wantError: context.Canceled},
		{name: "context deadline exceeded", err: context.DeadlineExceeded, wantError: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				calls.Add(1)
				return nil, tt.err
			})

			cl, err := New("https://accrual.com")
			require.NoError(t, err)
			cl.httpClient = &http.Client{Transport: transport}

			// Act
			_, err = cl.GetAccrual(context.Background(), "12345678903")

			// Assert
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantError)
			assert.Equal(t, int32(1), calls.Load())
		})
	}
}
