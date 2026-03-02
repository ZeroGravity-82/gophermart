package service

import (
	"context"
	"fmt"
	"time"

	"zerogravity-82/gophermart/internal/model"
)

// BalanceService содержит бизнес-логику работы с балансом и списаниями баллов.
type BalanceService struct {
	repo balanceRepository
}

type balanceRepository interface {
	GetBalance(ctx context.Context, userID string) (model.Balance, error)
	ListWithdrawals(ctx context.Context, userID string) ([]model.Withdrawal, error)
	CreateWithdrawal(ctx context.Context, userID, orderNumber string, sum uint64, now time.Time) error
}

// NewBalanceService создает BalanceService.
func NewBalanceService(repo balanceRepository) *BalanceService {
	return &BalanceService{repo: repo}
}

// GetBalance возвращает агрегированный баланс пользователя.
func (s *BalanceService) GetBalance(ctx context.Context, userID string) (model.Balance, error) {
	balance, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		return model.Balance{}, fmt.Errorf("failed to get balance: %w", err)
	}
	return balance, err
}

// ListWithdrawals возвращает информацию о выводе средств.
func (s *BalanceService) ListWithdrawals(ctx context.Context, userID string) ([]model.Withdrawal, error) {
	withdrawals, err := s.repo.ListWithdrawals(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list withdrawals: %w", err)
	}
	return withdrawals, err
}

// Withdraw регистрирует списание балов.
func (s *BalanceService) Withdraw(ctx context.Context, userID, orderNumber string, sum uint64) error {
	if !validLuhn(orderNumber) {
		return model.ErrInvalidOrderNumber
	}
	if sum == 0 {
		return model.ErrInvalidWithdrawalSum
	}

	err := s.repo.CreateWithdrawal(ctx, userID, orderNumber, sum, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to withdraw: %w", err)
	}
	return nil
}
