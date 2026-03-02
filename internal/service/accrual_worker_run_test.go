package service

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/accrual"
	"zerogravity-82/gophermart/internal/model"
)

type accrualClientStub struct {
	get func(ctx context.Context, orderNumber string) (accrual.GetAccrualAPIResponse, error)
}

func (s accrualClientStub) GetAccrual(ctx context.Context, orderNumber string) (accrual.GetAccrualAPIResponse, error) {
	return s.get(ctx, orderNumber)
}

type orderAccrualRepoStub struct {
	listFn  func(ctx context.Context, limit int) ([]model.Order, error)
	applyFn func(ctx context.Context, orderNumber string, status model.OrderStatus, accrualAmount *uint64) error

	applied []struct {
		n string
		s model.OrderStatus
		a *uint64
	}
}

func (s *orderAccrualRepoStub) ListForAccrualProcessing(ctx context.Context, limit int) ([]model.Order, error) {
	return s.listFn(ctx, limit)
}

func (s *orderAccrualRepoStub) ApplyAccrualResult(
	ctx context.Context,
	orderNumber string,
	status model.OrderStatus,
	accrualAmount *uint64,
) error {
	s.applied = append(s.applied, struct {
		n string
		s model.OrderStatus
		a *uint64
	}{n: orderNumber, s: status, a: accrualAmount})
	if s.applyFn != nil {
		return s.applyFn(ctx, orderNumber, status, accrualAmount)
	}
	return nil
}

// TestAccrualWorker_Run_StopsOnContextCancel проверяет, что Run завершается при отмене контекста.
func TestAccrualWorker_Run_StopsOnContextCancel(t *testing.T) {
	// Arrange
	repo := &orderAccrualRepoStub{listFn: func(_ context.Context, _ int) ([]model.Order, error) {
		return nil, nil
	}}
	cl := accrualClientStub{get: func(ctx context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		<-ctx.Done()
		return accrual.GetAccrualAPIResponse{}, ctx.Err()
	}}

	w, err := NewAccrualWorker(repo, cl, 10*time.Millisecond, 10, 2)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()

	// Act
	cancel()

	// Assert
	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatal("worker Run did not stop after context cancellation")
	}
}

// TestAccrualWorker_Run_RespectsWorkersLimit проверяет, что одновременно выполняется не больше workers запросов.
func TestAccrualWorker_Run_RespectsWorkersLimit(t *testing.T) {
	// Arrange
	const (
		workers   = 3
		batchSize = 10
	)

	orders := make([]model.Order, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		orders = append(orders, model.Order{Number: strconv.Itoa(i + 1)})
	}

	repo := &orderAccrualRepoStub{listFn: func(_ context.Context, _ int) ([]model.Order, error) {
		return orders, nil
	}}

	// gate блокирует вызовы GetAccrual, пока мы их не разблокируем.
	gate := make(chan struct{})

	var inFlight int64
	var maxInFlight int64

	cl := accrualClientStub{get: func(ctx context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		cur := atomic.AddInt64(&inFlight, 1)
		for {
			prev := atomic.LoadInt64(&maxInFlight)
			if cur <= prev {
				break
			}
			if atomic.CompareAndSwapInt64(&maxInFlight, prev, cur) {
				break
			}
		}

		select {
		case <-ctx.Done():
			atomic.AddInt64(&inFlight, -1)
			return accrual.GetAccrualAPIResponse{}, ctx.Err()
		case <-gate:
			atomic.AddInt64(&inFlight, -1)
			return accrual.GetAccrualAPIResponse{Order: "1", Status: "REGISTERED"}, nil
		}
	}}

	w, err := NewAccrualWorker(repo, cl, 5*time.Millisecond, batchSize, workers)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()

	// Act
	// Даем времени задачам попасть в очередь и быть взятыми воркерами.
	time.Sleep(50 * time.Millisecond)

	// Assert
	// В этот момент пул должен быть загружен, но не превышать лимит.
	assert.LessOrEqual(t, atomic.LoadInt64(&maxInFlight), int64(workers))

	// Разблокируем текущее количество параллельных воркеров.
	for i := 0; i < workers; i++ {
		gate <- struct{}{}
	}

	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatal("worker Run did not stop after context cancellation")
	}
}

// TestAccrualWorker_429_SleepsAllWorkers проверяет, что при 429 все воркеры начинают спать.
func TestAccrualWorker_429_SleepsAllWorkers(t *testing.T) {
	// Arrange
	retryAfter := 200 * time.Millisecond

	orders := []model.Order{{Number: "1"}, {Number: "2"}, {Number: "3"}, {Number: "4"}}
	repo := &orderAccrualRepoStub{listFn: func(_ context.Context, _ int) ([]model.Order, error) {
		return orders, nil
	}}

	// First request returns 429 with retry-after; subsequent requests block on gate.
	var calls atomic.Int64
	gate := make(chan struct{})

	cl := accrualClientStub{get: func(ctx context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		n := calls.Add(1)
		if n == 1 {
			return accrual.GetAccrualAPIResponse{}, accrual.ErrTooManyRequests{RetryAfter: retryAfter}
		}
		select {
		case <-ctx.Done():
			return accrual.GetAccrualAPIResponse{}, ctx.Err()
		case <-gate:
			return accrual.GetAccrualAPIResponse{Order: "1", Status: "REGISTERED"}, nil
		}
	}}

	w, err := NewAccrualWorker(repo, cl, 5*time.Millisecond, 10, 2)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()
	defer cancel()

	// Act
	// Даем первому ответу 429 шанс быть обработанным.
	time.Sleep(30 * time.Millisecond)

	// Assert
	// sleepUntil должен быть выставлен хотя бы в now+retryAfter.
	sleepUntil := time.Unix(0, w.sleepUntil.Load())
	assert.Greater(t, sleepUntil, time.Now().Add(100*time.Millisecond))

	cancel()
	close(gate)
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not stop")
	}
}

// TestAccrualWorker_429_GracefulShutdownDuringSleep проверяет, что shutdown работает даже во время сна.
func TestAccrualWorker_429_GracefulShutdownDuringSleep(t *testing.T) {
	// Arrange
	retryAfter := 2 * time.Second

	repo := &orderAccrualRepoStub{listFn: func(_ context.Context, _ int) ([]model.Order, error) {
		return []model.Order{{Number: "1"}}, nil
	}}
	cl := accrualClientStub{get: func(_ context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		return accrual.GetAccrualAPIResponse{}, accrual.ErrTooManyRequests{RetryAfter: retryAfter}
	}}

	w, err := NewAccrualWorker(repo, cl, 5*time.Millisecond, 10, 2)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()

	// Act
	// Ждем, пока sleepUntil будет выставлен.
	require.Eventually(t, func() bool {
		return w.sleepUntil.Load() != 0
	}, 300*time.Millisecond, 10*time.Millisecond)
	cancel()

	// Assert
	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not stop during backoff sleep")
	}
}

// TestNewAccrualWorker_RejectsNonPositiveWorkers проверяет валидацию параметра workers.
func TestNewAccrualWorker_RejectsNonPositiveWorkers(t *testing.T) {
	// Arrange
	repo := &orderAccrualRepoStub{listFn: func(_ context.Context, _ int) ([]model.Order, error) { return nil, nil }}
	cl := accrualClientStub{get: func(_ context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		return accrual.GetAccrualAPIResponse{}, nil
	}}

	// Act
	_, err := NewAccrualWorker(repo, cl, 1*time.Second, 10, 0)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workers")
}

// TestAccrualWorker_ProcessOrders_ListError проверяет ошибку при получении списка заказов.
func TestAccrualWorker_ProcessOrders_ListError(t *testing.T) {
	// Arrange
	repo := &orderAccrualRepoStub{listFn: func(_ context.Context, _ int) ([]model.Order, error) {
		return nil, errors.New("db down")
	}}
	cl := accrualClientStub{get: func(_ context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		return accrual.GetAccrualAPIResponse{}, nil
	}}
	w, err := NewAccrualWorker(repo, cl, 1*time.Second, 10, 5)
	require.NoError(t, err)
	orderNumberCh := make(chan string)

	// Act
	err = w.processOrdersWithAccrual(context.Background(), orderNumberCh)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list orders for accrual")
}

// TestAccrualWorker_ProcessOrder_OrderNotProcessed проверяет игнорирование ErrOrderNotProcessed.
func TestAccrualWorker_ProcessOrder_OrderNotProcessed(t *testing.T) {
	// Arrange
	repo := &orderAccrualRepoStub{}
	cl := accrualClientStub{get: func(_ context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		return accrual.GetAccrualAPIResponse{}, accrual.ErrOrderNotProcessed
	}}
	w, err := NewAccrualWorker(repo, cl, 1*time.Second, 10, 5)
	require.NoError(t, err)

	// Act
	err = w.processOrder(context.Background(), "1")

	// Assert
	require.NoError(t, err)
	assert.Len(t, repo.applied, 0)
}

// TestAccrualWorker_ProcessOrder_AppliesResult проверяет применение результата к репозиторию.
func TestAccrualWorker_ProcessOrder_AppliesResult(t *testing.T) {
	// Arrange
	repo := &orderAccrualRepoStub{}
	accrualMajor := 1.01
	cl := accrualClientStub{get: func(_ context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		return accrual.GetAccrualAPIResponse{Order: "1", Status: "PROCESSED", Accrual: &accrualMajor}, nil
	}}
	w, err := NewAccrualWorker(repo, cl, 1*time.Second, 10, 5)
	require.NoError(t, err)

	// Act
	err = w.processOrder(context.Background(), "1")

	// Assert
	require.NoError(t, err)
	require.Len(t, repo.applied, 1)
	assert.Equal(t, "1", repo.applied[0].n)
	assert.Equal(t, model.OrderStatusProcessed, repo.applied[0].s)
	require.NotNil(t, repo.applied[0].a)
	assert.EqualValues(t, 101, *repo.applied[0].a)
}
