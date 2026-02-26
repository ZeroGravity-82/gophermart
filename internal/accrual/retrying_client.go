package accrual

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// retryingHTTPClient - обертка над retryablehttp.Client, которая предоставляет метод Do в стиле http.Client.
//
// Политика повторной отправки запросов:
// - повторяем запросы при сетевых/транспортных ошибках (кроме отмены/истечения дедлайна контекста);
// - повторяем запросы при ответах 5xx;
// - повторяем запросы при ответах 429 (с учетом Retry-After) и 408;
// - не повторяем запросы при прочих ответах 4xx.
//
// Обертка сделана максимально общей: она ничего не знает про JSON, конкретные ручки или доменные ошибки.
// Решение о повторной отправке запросов на уровне домена (например, из-за некорректного формата ответа) должно
// приниматься вызывающим кодом.
type retryingHTTPClient struct {
	client *retryablehttp.Client
}

func newRetryingHTTPClient(
	httpClientTimeout time.Duration,
	maxRetries uint,
	minBackoff, maxBackoff, perAttemptTimeout time.Duration,
) *retryingHTTPClient {
	hc := &http.Client{Timeout: httpClientTimeout}
	rc := retryablehttp.NewClient()
	rc.HTTPClient = hc
	rc.RetryMax = int(maxRetries)
	rc.RetryWaitMin = minBackoff
	rc.RetryWaitMax = maxBackoff
	// Отключаем болтливый логгер retryablehttp.
	rc.Logger = nil

	// Расширяем политику повторной отправки запросов.
	defaultCheck := rc.CheckRetry
	rc.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return false, err
			}
		}
		if resp != nil && resp.StatusCode == http.StatusRequestTimeout {
			return true, nil
		}
		return defaultCheck(ctx, resp, err)
	}

	// На каждую HTTP-попытку создаем контекст с дедлайном.
	// Важно: это тайм-аут каждой попытки, а не общий тайм-аут на всю цепочку попыток.
	rc.RequestLogHook = func(_ retryablehttp.Logger, r *http.Request, _ int) {
		if perAttemptTimeout <= 0 {
			return
		}

		ctx := r.Context()
		// Если для контекста уже задан дедлайн, не добавляем дополнительных ограничений по времени.
		if _, ok := ctx.Deadline(); ok {
			return
		}

		attemptCtx, cancel := context.WithTimeout(ctx, perAttemptTimeout)
		// retryablehttp не дает "post-attempt" хука, поэтому вызываем cancel после завершения контекста.
		go func() {
			<-attemptCtx.Done()
			cancel()
		}()

		*r = *r.WithContext(attemptCtx)
	}

	// Поддержка заголовка Retry-After для ответов 429.
	defaultBackoff := rc.Backoff
	rc.Backoff = func(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
		if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, convErr := strconv.Atoi(strings.TrimSpace(ra)); convErr == nil && secs > 0 {
					return time.Duration(secs) * time.Second
				}
			}
		}
		return defaultBackoff(min, max, attemptNum, resp)
	}

	return &retryingHTTPClient{client: rc}
}

func (c *retryingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// retryablehttp может отправлять запрос повторно. Тело запроса должно быть перечитываемым.
	if req.Body != nil {
		b, readErr := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		req.Body = io.NopCloser(bytes.NewReader(b))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(b)), nil
		}
	}

	// Транспортный слой делает повторы, но тайм-аут каждой попытки задается на уровне запроса через контекст
	// (см. RequestLogHook выше).
	rreq, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, err
	}

	return c.client.Do(rreq)
}
