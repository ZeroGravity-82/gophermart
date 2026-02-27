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

	// Gate blocks GetAccrual calls until we release them.
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
	// Let some tasks get queued and picked up.
	time.Sleep(50 * time.Millisecond)

	// Assert
	// At this point the pool should be saturated, but not exceed limit.
	assert.LessOrEqual(t, atomic.LoadInt64(&maxInFlight), int64(workers))

	// Release enough times to unblock currently running workers.
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
