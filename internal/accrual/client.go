// Package accrual содержит HTTP-клиент для взаимодействия с внешним сервисом расчета начислений баллов лояльности.
package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	urlGetAccrualPrefix string = "/api/orders"
	maxRetries          uint   = 3

	// requestTimeout задает тайм-аут на каждую попытку отправки запроса.
	requestTimeout time.Duration = 30 * time.Second

	// httpClientTimeout является подстраховкой от зависаний транспорта/чтения тела ответа.
	httpClientTimeout time.Duration = 60 * time.Second
)

// Client является HTTP-клиентом API для сервиса расчета начислений баллов лояльности.
//
// Повторные запросы и экспоненциальная задержка (backoff) реализованы во внутреннем транспортном клиенте.
type Client struct {
	httpClient *retryingHTTPClient
	baseURL    string
}

// New создает новый экземпляр Client.
//
// baseURL представляет собой базовый адрес сервиса расчета начислений баллов лояльности (например, http://accrual.com).
func New(baseURL string) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("baseURL is empty")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid baseURL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid baseURL: schema or host is not provided")
	}

	return &Client{
		httpClient: newRetryingHTTPClient(
			httpClientTimeout,
			maxRetries,
			200*time.Millisecond,
			2*time.Second,
			requestTimeout,
		),
		baseURL: baseURL,
	}, nil
}

// GetAccrualAPIResponse описывает ответ, возвращаемый API сервиса расчета начислений баллов лояльности.
//
// Order содержит номер заказа.
//
// Status представляет собой статус расчета начисления.
//
// Accrual содержит рассчитанные баллы к начислению, при отсутствии начисления это поле отсутствует в ответе.
type GetAccrualAPIResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

// ErrOrderNotProcessed возвращается когда заказ не зарегистрирован в сервисе расчета начислений баллов лояльности, и
// он вернул код HTTP-ответа 204.
var ErrOrderNotProcessed = errors.New("order is not processed in accrual system yet")

// ErrTooManyRequests возвращается, когда сервис начислений отвечает 429 Too Many Requests.
// RetryAfter содержит время, которое рекомендуется выждать перед следующей попыткой.
//
// Retry-After может быть отсутствующим или некорректным, тогда RetryAfter будет 0.
type ErrTooManyRequests struct {
	RetryAfter time.Duration
}

func (e ErrTooManyRequests) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("too many requests, retry after %s", e.RetryAfter)
	}
	return "too many requests"
}

// GetAccrual вызывает метод `GET /api/orders/{number}` сервиса расчета начислений баллов лояльности.
func (c *Client) GetAccrual(ctx context.Context, orderNumber string) (GetAccrualAPIResponse, error) {
	orderNumber = strings.TrimSpace(orderNumber)
	if orderNumber == "" {
		return GetAccrualAPIResponse{}, errors.New("orderNumber is empty")
	}

	urlFull, err := url.JoinPath(c.baseURL, urlGetAccrualPrefix, orderNumber)
	if err != nil {
		return GetAccrualAPIResponse{}, fmt.Errorf("failed to build full URL during getting accrual: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, urlFull, nil)
	if err != nil {
		return GetAccrualAPIResponse{}, fmt.Errorf("failed to build request during getting accrual: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return GetAccrualAPIResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Код 204 означает, что заказ не зарегистрирован в сервисе расчета начислений баллов лояльности,
	// т.е. еще не обработан.
	if resp.StatusCode == http.StatusNoContent {
		return GetAccrualAPIResponse{}, ErrOrderNotProcessed
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		var retryAfter time.Duration
		if ra := strings.TrimSpace(resp.Header.Get("Retry-After")); ra != "" {
			if secs, convErr := strconv.Atoi(ra); convErr == nil && secs > 0 {
				retryAfter = time.Duration(secs) * time.Second
			}
		}
		return GetAccrualAPIResponse{}, ErrTooManyRequests{RetryAfter: retryAfter}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return GetAccrualAPIResponse{}, err
	}

	var apiResp GetAccrualAPIResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return GetAccrualAPIResponse{}, fmt.Errorf(
				"decode response failure (status=%d) during getting accrual: %w",
				resp.StatusCode,
				err,
			)
		}
	}

	if resp.StatusCode != http.StatusOK {
		return GetAccrualAPIResponse{}, fmt.Errorf("accrual API error (status=%d): %v", resp.StatusCode, apiResp)
	}

	return apiResp, nil
}
