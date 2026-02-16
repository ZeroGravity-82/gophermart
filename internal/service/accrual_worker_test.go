package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/accrual"
	"zerogravity-82/gophermart/internal/model"
)

// TestMapAccrualResponse_ProcessedNilAccrualDoesNotPanic проверяет, что nil-значение accrual не вызывает панику.
func TestMapAccrualResponse_ProcessedNilAccrualDoesNotPanic(t *testing.T) {
	// Arrange
	req := accrual.GetAccrualAPIResponse{
		Order:   "1",
		Status:  "PROCESSED",
		Accrual: nil,
	}
	defer func() {
		recovered := recover()
		assert.Nil(t, recovered)
	}()

	// Act
	status, amount, err := mapAccrualResponse(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, model.OrderStatusProcessed, status)
	assert.NotNil(t, amount)
	assert.Equal(t, uint64(0), *amount)
}

// TestMapAccrualResponse_ProcessedWithAccrualRoundsToMinorUnits проверяет округление начисления до минорных единиц.
func TestMapAccrualResponse_ProcessedWithAccrualRoundsToMinorUnits(t *testing.T) {
	// Arrange
	v := 1.01
	req := accrual.GetAccrualAPIResponse{
		Order:   "1",
		Status:  "PROCESSED",
		Accrual: &v,
	}

	// Act
	status, amount, err := mapAccrualResponse(req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, model.OrderStatusProcessed, status)
	assert.NotNil(t, amount)
	assert.Equal(t, uint64(101), *amount)
}
