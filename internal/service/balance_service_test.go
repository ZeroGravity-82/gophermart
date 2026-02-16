package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/model"
)

type balanceRepoStub struct {
	balance     model.Balance
	withdrawals []model.Withdrawal
	withdrawErr error
	getErr      error

	lastOrder string
	lastSum   uint64
}

func (s *balanceRepoStub) GetBalance(_ context.Context, _ string) (model.Balance, error) {
	return s.balance, s.getErr
}

func (s *balanceRepoStub) ListWithdrawals(_ context.Context, _ string) ([]model.Withdrawal, error) {
	return s.withdrawals, s.getErr
}

func (s *balanceRepoStub) CreateWithdrawal(
	_ context.Context,
	_ string,
	orderNumber string,
	sum uint64,
	_ time.Time,
) error {
	s.lastOrder = orderNumber
	s.lastSum = sum
	if s.withdrawErr != nil {
		return s.withdrawErr
	}
	return nil
}

// TestBalanceService_Withdraw_InvalidOrder проверяет ошибку при некорректном номере заказа.
func TestBalanceService_Withdraw_InvalidOrder(t *testing.T) {
	// Arrange
	repo := &balanceRepoStub{}
	svc := NewBalanceService(repo)

	// Act
	err := svc.Withdraw(context.Background(), "user-1", "123", 100)

	// Assert
	assert.ErrorIs(t, err, model.ErrInvalidOrderNumber)
}

// TestBalanceService_Withdraw_InvalidSum проверяет ошибку при некорректной сумме списания.
func TestBalanceService_Withdraw_InvalidSum(t *testing.T) {
	// Arrange
	repo := &balanceRepoStub{}
	svc := NewBalanceService(repo)

	// Act
	err := svc.Withdraw(context.Background(), "user-1", "79927398713", 0)

	// Assert
	assert.ErrorIs(t, err, model.ErrInvalidWithdrawalSum)
}

// TestBalanceService_Withdraw_PropagatesRepoError проверяет проброс ошибки из репозитория.
func TestBalanceService_Withdraw_PropagatesRepoError(t *testing.T) {
	// Arrange
	repo := &balanceRepoStub{withdrawErr: model.ErrInsufficientFunds}
	svc := NewBalanceService(repo)

	// Act
	err := svc.Withdraw(context.Background(), "user-1", "79927398713", 100)

	// Assert
	assert.ErrorIs(t, err, model.ErrInsufficientFunds)
}

// TestBalanceService_Withdraw_Ok проверяет успешное списание.
func TestBalanceService_Withdraw_Ok(t *testing.T) {
	// Arrange
	repo := &balanceRepoStub{}
	svc := NewBalanceService(repo)

	// Act
	err := svc.Withdraw(context.Background(), "user-1", "79927398713", 123)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "79927398713", repo.lastOrder)
	assert.Equal(t, uint64(123), repo.lastSum)
}
