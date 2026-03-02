package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"zerogravity-82/gophermart/internal/model"
)

// TestToOrderResponse_WithNilAccrual проверяет преобразование заказа без начисления.
func TestToOrderResponse_WithNilAccrual(t *testing.T) {
	// Arrange
	now := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	order := model.Order{Number: "1", Status: model.OrderStatusNew, UploadedAt: now, Accrual: nil}

	// Act
	resp := ToOrderResponse(order)

	// Assert
	assert.Equal(t, "1", resp.Number)
	assert.Equal(t, string(model.OrderStatusNew), resp.Status)
	assert.Equal(t, now, resp.UploadedAt)
	assert.Nil(t, resp.Accrual)
}

// TestToOrderResponse_WithAccrual проверяет преобразование заказа с начислением.
func TestToOrderResponse_WithAccrual(t *testing.T) {
	// Arrange
	now := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	accrualMinor := uint64(12345)
	order := model.Order{Number: "1", Status: model.OrderStatusProcessed, UploadedAt: now, Accrual: &accrualMinor}

	// Act
	resp := ToOrderResponse(order)

	// Assert
	assert.NotNil(t, resp.Accrual)
	assert.Equal(t, float64(accrualMinor)/model.AccrualScale, *resp.Accrual)
}

// TestToWithdrawalResponse проверяет преобразование списания в ответ API.
func TestToWithdrawalResponse(t *testing.T) {
	// Arrange
	now := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	w := model.Withdrawal{OrderNumber: "123", Sum: 777, ProcessedAt: now}

	// Act
	resp := ToWithdrawalResponse(w)

	// Assert
	assert.Equal(t, "123", resp.Order)
	assert.Equal(t, float64(777)/model.AccrualScale, resp.Sum)
	assert.Equal(t, now, resp.ProcessedAt)
}
