package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"zerogravity-82/gophermart/internal/model"
)

// OrderRepo реализует доступ к заказам, хранящимся в PostgreSQL.
type OrderRepo struct {
	db      *sqlx.DB
	lockTTL time.Duration
}

// NewOrderRepo создает OrderRepo на основе подключения к БД.
func NewOrderRepo(db *sqlx.DB, lockTTL time.Duration) (*OrderRepo, error) {
	if lockTTL <= 0 {
		return nil, errors.New("lock TTL must be positive")
	}
	return &OrderRepo{db: db, lockTTL: lockTTL}, nil
}

// ListForAccrualProcessing возвращает заказы, которые еще не имеют окончательных статусов (т.е. заказы со статусом NEW
// и PROCESSING).
func (r *OrderRepo) ListForAccrualProcessing(ctx context.Context, limit int) ([]model.Order, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	orders, err := r.getOrdersWithLockForAccrualProcessing(ctx, tx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders for accrual processing: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return orders, nil
}

func (r *OrderRepo) getOrdersWithLockForAccrualProcessing(
	ctx context.Context,
	tx *sqlx.Tx,
	limit int,
) ([]model.Order, error) {
	const qSelectWithLock = `
SELECT number, status, uploaded_at, accrual
	FROM "order"
	WHERE status IN ($1, $2)
	  AND (lock_expires_at IS NULL OR lock_expires_at <= NOW())
	ORDER BY uploaded_at ASC
	LIMIT $3
	FOR UPDATE SKIP LOCKED
`

	var orders []model.Order
	if err := tx.SelectContext(
		ctx,
		&orders,
		qSelectWithLock,
		model.OrderStatusNew,
		model.OrderStatusProcessing,
		limit,
	); err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return orders, nil
	}

	err := r.lockOrdersForAccrualProcessing(ctx, tx, orders)
	if err != nil {
		return nil, err
	}

	return orders, nil
}

func (r *OrderRepo) lockOrdersForAccrualProcessing(ctx context.Context, tx *sqlx.Tx, orders []model.Order) error {
	numbers := make([]string, 0, len(orders))
	for _, o := range orders {
		numbers = append(numbers, o.Number)
	}

	// sqlx.In умеет расширять только плейсхолдеры `?`.
	qLock, args, err := sqlx.In(
		`UPDATE "order" SET lock_expires_at = NOW() + ? WHERE number IN (?)`,
		r.lockTTL,
		numbers,
	)
	if err != nil {
		return err
	}
	qLock = tx.Rebind(qLock)
	if _, err := tx.ExecContext(ctx, qLock, args...); err != nil {
		return err
	}
	return nil
}

// ApplyAccrualResult обновляет статус/начисление заказа.
//
// Если статус PROCESSED и accrualAmount == nil, то accrual устанавливается в 0.
func (r *OrderRepo) ApplyAccrualResult(
	ctx context.Context,
	orderNumber string,
	status model.OrderStatus,
	accrualAmount *uint64,
) error {
	// Делаем обновление идемпотентным: не обновляем заказ, у которого уже окончательный статус.
	const q = `UPDATE "order"
	SET status = $1, accrual = $2
	WHERE number = $3
	  AND status IN ($4, $5)`

	var accrualPtr *int64
	if accrualAmount != nil {
		v := int64(*accrualAmount)
		accrualPtr = &v
	}

	res, err := r.db.ExecContext(
		ctx,
		q,
		status,
		accrualPtr,
		orderNumber,
		model.OrderStatusNew,
		model.OrderStatusProcessing,
	)
	if err != nil {
		return fmt.Errorf("failed to update order accrual: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil
	}
	return nil
}

// Create создает заказ в БД для указанного пользователя.
//
// Если заказ с таким номером уже существует, возвращает одну из ошибок:
// model.ErrOrderAlreadyExistsSameUser или model.ErrOrderAlreadyExistsOtherUser.
func (r *OrderRepo) Create(ctx context.Context, userID string, o model.Order) error {
	const qInsert = `INSERT INTO "order" (number, user_id, status, uploaded_at) VALUES ($1, $2, $3, $4)`
	_, err := r.db.ExecContext(ctx, qInsert, o.Number, userID, o.Status, o.UploadedAt)
	if err == nil {
		return nil
	}

	if !isUniqueViolation(err) {
		return fmt.Errorf("failed to persist new order: %w", err)
	}

	// Determine whether the existing order belongs to this user.
	const qOwner = `SELECT user_id FROM "order" WHERE number = $1`
	var owner string
	if qErr := r.db.GetContext(ctx, &owner, qOwner, o.Number); qErr != nil {
		return fmt.Errorf("failed to persist new order: %w", qErr)
	}
	if owner == userID {
		return model.ErrOrderAlreadyExistsSameUser
	}
	return model.ErrOrderAlreadyExistsOtherUser
}

// ListByUserID возвращает список загруженных номеров заказов для указанного пользователя. Список отсортирован по
// времени загрузки (от самых новых к самым старым).
func (r *OrderRepo) ListByUserID(ctx context.Context, userID string) ([]model.Order, error) {
	const q = `SELECT number, status, uploaded_at, accrual FROM "order" WHERE user_id = $1 ORDER BY uploaded_at DESC`
	var orders []model.Order
	if err := r.db.SelectContext(ctx, &orders, q, userID); err != nil {
		return nil, fmt.Errorf("failed to select orders: %w", err)
	}
	return orders, nil
}
