package service

import (
	"context"
	"errors"
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

// TestAccrualWorker_ProcessOrders_ListError проверяет ошибку при получении списка заказов.
func TestAccrualWorker_ProcessOrders_ListError(t *testing.T) {
	// Arrange
	repo := &orderAccrualRepoStub{listFn: func(_ context.Context, _ int) ([]model.Order, error) {
		return nil, errors.New("db down")
	}}
	cl := accrualClientStub{get: func(_ context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		return accrual.GetAccrualAPIResponse{}, nil
	}}
	w, err := NewAccrualWorker(repo, cl, 1*time.Second, 10)
	require.NoError(t, err)

	// Act
	err = w.processOrdersWithAccrual(context.Background())

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
	w, err := NewAccrualWorker(repo, cl, 1*time.Second, 10)
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
	w, err := NewAccrualWorker(repo, cl, 1*time.Second, 10)
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

// TestAccrualWorker_ProcessOrders_StopsOnFirstError проверяет, что воркер останавливается на первой ошибке обработки.
func TestAccrualWorker_ProcessOrders_StopsOnFirstError(t *testing.T) {
	// Arrange
	repo := &orderAccrualRepoStub{listFn: func(_ context.Context, _ int) ([]model.Order, error) {
		return []model.Order{{Number: "1"}, {Number: "2"}}, nil
	}}
	cl := accrualClientStub{get: func(_ context.Context, order string) (accrual.GetAccrualAPIResponse, error) {
		if order == "1" {
			return accrual.GetAccrualAPIResponse{}, errors.New("network")
		}
		return accrual.GetAccrualAPIResponse{Order: order, Status: "REGISTERED"}, nil
	}}
	w, err := NewAccrualWorker(repo, cl, 1*time.Second, 10)
	require.NoError(t, err)

	// Act
	err = w.processOrdersWithAccrual(context.Background())

	// Assert
	require.Error(t, err)
	assert.Len(t, repo.applied, 0)
}
