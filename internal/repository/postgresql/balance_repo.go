package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"zerogravity-82/gophermart/internal/model"
)

// BalanceRepo реализует доступ к данным баланса пользователя в PostgreSQL.
type BalanceRepo struct {
	db *sqlx.DB
}

// NewBalanceRepo создает BalanceRepo на основе подключения к БД.
func NewBalanceRepo(db *sqlx.DB) *BalanceRepo {
	return &BalanceRepo{db: db}
}

// GetBalance возвращает агрегированный баланс пользователя.
func (r *BalanceRepo) GetBalance(ctx context.Context, userID string) (model.Balance, error) {
	const q = `
SELECT COALESCE((SELECT SUM(o.accrual)
                 FROM "order" o
                 WHERE o.user_id = $1 AND o.status = $2 AND o.accrual IS NOT NULL), 0) AS accrual,
       COALESCE((SELECT SUM(w.sum) FROM withdrawal w WHERE w.user_id = $1), 0)         AS withdrawn
`
	var b model.Balance
	if err := r.db.GetContext(ctx, &b, q, userID, model.OrderStatusProcessed); err != nil {
		return model.Balance{}, fmt.Errorf("failed to select accrual or withdrawal data: %w", err)
	}
	return b, nil
}

// ListWithdrawals возвращает список списаний пользователя (по убыванию даты).
func (r *BalanceRepo) ListWithdrawals(ctx context.Context, userID string) ([]model.Withdrawal, error) {
	const q = `SELECT order_number, sum, processed_at FROM withdrawal WHERE user_id = $1 ORDER BY processed_at DESC`
	var withdrawals []model.Withdrawal
	if err := r.db.SelectContext(ctx, &withdrawals, q, userID); err != nil {
		return nil, fmt.Errorf("failed to select withdrawals: %w", err)
	}
	return withdrawals, nil
}

// CreateWithdrawal списывает баллы лояльности в счет оплаты заказа.
func (r *BalanceRepo) CreateWithdrawal(
	ctx context.Context,
	userID, orderNumber string,
	sum uint64,
	now time.Time,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Получаем агрегированный баланс пользователя.
	balance, err := getBalanceWithLockForUpdate(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to get balance with lock for update: %w", err)
	}
	current := balance.Accrual - balance.Withdrawn
	if current < sum {
		return model.ErrInsufficientFunds
	}

	// Списываем баллы лояльности в счет оплаты заказа.
	const q = `INSERT INTO withdrawal (order_number, user_id, sum, processed_at) VALUES ($1, $2, $3, $4)`
	if _, err = tx.ExecContext(ctx, q, orderNumber, userID, sum, now); err != nil {
		if isUniqueViolation(err) {
			return model.ErrWithdrawConflict
		}
		return fmt.Errorf("failed to persist new withdrawal: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// getBalance вычисляет агрегированный баланс пользователя.
func getBalanceWithLockForUpdate(ctx context.Context, tx *sqlx.Tx, userID string) (model.Balance, error) {
	// Сериализуем параллельные списания для одного и того же пользователя.
	const qLock = `SELECT 1 FROM "user" WHERE id = $1 FOR UPDATE`
	var dummy int
	if err := tx.GetContext(ctx, &dummy, qLock, userID); err != nil {
		return model.Balance{}, err
	}

	const q = `
SELECT COALESCE((SELECT SUM(o.accrual)
			     FROM "order" o
			  	 WHERE o.user_id = $1 AND o.status = $2 AND o.accrual IS NOT NULL), 0) AS accrual,
	   COALESCE((SELECT SUM(w.sum) FROM withdrawal w WHERE w.user_id = $1), 0)         AS withdrawn
`
	var b model.Balance
	if err := tx.GetContext(ctx, &b, q, userID, model.OrderStatusProcessed); err != nil {
		return model.Balance{}, err
	}
	return b, nil
}
