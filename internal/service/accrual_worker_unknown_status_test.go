package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/accrual"
	"zerogravity-82/gophermart/internal/model"
)

// TestAccrualWorker_ProcessOrder_UnknownStatus проверяет ошибку при неизвестном статусе начислений.
func TestAccrualWorker_ProcessOrder_UnknownStatus(t *testing.T) {
	// Arrange
	repo := &orderAccrualRepoStub{listFn: func(_ context.Context, _ int) ([]model.Order, error) { return nil, nil }}
	cl := accrualClientStub{get: func(_ context.Context, _ string) (accrual.GetAccrualAPIResponse, error) {
		return accrual.GetAccrualAPIResponse{Order: "1", Status: "WTF"}, nil
	}}
	w, err := NewAccrualWorker(repo, cl, 1*time.Second, 10, 5)
	require.NoError(t, err)
	ctx := context.Background()

	// Act
	err = w.processOrder(ctx, "1")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown accrual status")
}
