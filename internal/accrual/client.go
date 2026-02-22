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

	"github.com/cenkalti/backoff/v5"
)

const (
	urlGetAccrualPrefix string        = "/api/orders"
	maxRetries          uint          = 3
	requestTimeout      time.Duration = 30 * time.Second

	// httpClientTimeout является подстраховкой от зависаний транспорта/чтения тела ответа.
	httpClientTimeout time.Duration = 60 * time.Second
)

// Client является HTTP-клиентом API для сервиса расчета начислений баллов лояльности.
//
// Он поддерживает экспоненциальную задержку при отправке повторных запросов в случае ошибок.
type Client struct {
	httpClient *http.Client
	baseURL    string
	backoff    func() backoff.BackOff
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
		httpClient: &http.Client{Timeout: httpClientTimeout},
		baseURL:    baseURL,
		backoff: func() backoff.BackOff {
			b := backoff.NewExponentialBackOff()
			b.InitialInterval = 200 * time.Millisecond
			b.MaxInterval = 2 * time.Second
			return b
		},
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
// он вернула код HTTP-ответа 204.
var ErrOrderNotProcessed = errors.New("order is not processed in accrual system yet")

// GetAccrual вызывает метод `GET /api/orders/{number}` сервиса расчета начислений баллов лояльности.
//
// Повторно выполняются запросы при сетевых ошибках (за исключением отмены/истечении дедлайна контекста) и ответах с
// HTTP-кодом 5xx. Не выполняются повторно запросы при ответах с HTTP-кодом 4xx (кроме 429 и 408).
func (c *Client) GetAccrual(ctx context.Context, orderNumber string) (GetAccrualAPIResponse, error) {
	orderNumber = strings.TrimSpace(orderNumber)
	if orderNumber == "" {
		return GetAccrualAPIResponse{}, errors.New("orderNumber is empty")
	}

	op := c.doGetAccrual(ctx, orderNumber)
	data, err := backoff.Retry(ctx, op, backoff.WithBackOff(c.backoff()), backoff.WithMaxTries(maxRetries+1))
	if err != nil {
		return GetAccrualAPIResponse{}, fmt.Errorf("request failed: %w", err)
	}
	return data, nil
}

func (c *Client) doGetAccrual(ctx context.Context, orderNumber string) func() (GetAccrualAPIResponse, error) {
	return func() (GetAccrualAPIResponse, error) {
		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		urlFull, err := url.JoinPath(c.baseURL, urlGetAccrualPrefix, orderNumber)
		if err != nil {
			return GetAccrualAPIResponse{}, fmt.Errorf("failed to build full URL during getting accrual: %w", err)
		}
		httpReq, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, urlFull, nil)
		if err != nil {
			return GetAccrualAPIResponse{}, backoff.Permanent(
				fmt.Errorf("failed to build request during getting accrual: %w", err),
			)
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			// Не повторяем запрос при отмене/истечении дедлайна контекста.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return GetAccrualAPIResponse{}, backoff.Permanent(err)
			}
			return GetAccrualAPIResponse{}, err
		}
		defer resp.Body.Close()

		// Код 204 означает, что заказ не зарегистрирован в сервисе расчета начислений баллов лояльности,
		// т.е. еще не обработан.
		if resp.StatusCode == http.StatusNoContent {
			return GetAccrualAPIResponse{}, backoff.Permanent(ErrOrderNotProcessed)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return GetAccrualAPIResponse{}, err
		}

		var apiResp GetAccrualAPIResponse
		if len(body) > 0 {
			if err := json.Unmarshal(body, &apiResp); err != nil {
				wrapperErr := fmt.Errorf(
					"decode response failure (status=%d) during getting accrual: %w",
					resp.StatusCode,
					err,
				)

				// Не JSON-ответ - считаем повторяемой ошибкой только для HTTP-кода 5xx.
				if resp.StatusCode >= http.StatusInternalServerError {
					return GetAccrualAPIResponse{}, wrapperErr
				}
				return GetAccrualAPIResponse{}, backoff.Permanent(wrapperErr)
			}
		}

		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return apiResp, nil
		}

		err = fmt.Errorf("accrual API error (status=%d): %v", resp.StatusCode, apiResp)

		// Повторяем запрос только при серверной ошибке, клиентские ошибки не считаются повторяемыми (кроме 429 и 408).
		if resp.StatusCode == http.StatusTooManyRequests {
			// Учитываем заголовок Retry-After при его наличии.
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, convErr := strconv.Atoi(strings.TrimSpace(ra)); convErr == nil && secs > 0 {
					select {
					case <-time.After(time.Duration(secs) * time.Second):
						return GetAccrualAPIResponse{}, err
					case <-attemptCtx.Done():
						return GetAccrualAPIResponse{}, backoff.Permanent(attemptCtx.Err())
					}
				} else {
					err = fmt.Errorf("%w: invalid Retry-After header %q: %v", err, ra, convErr)
				}
			}
			return GetAccrualAPIResponse{}, err
		}
		if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusRequestTimeout {
			return GetAccrualAPIResponse{}, err
		}

		return GetAccrualAPIResponse{}, backoff.Permanent(err)
	}
}
