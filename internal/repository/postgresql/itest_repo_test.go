//go:build integration

package postgresql

// Файл содержит интеграционные тесты PostgreSQL-репозиториев.
//
// Тесты требуют реальную PostgreSQL и запускаются только при указании build-тега `integration`.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/model"
)

// TestUserRepo_CreateAndGetByLogin проверяет, что пользователь корректно сохраняется в PostgreSQL и затем извлекается
// по логину.
func TestUserRepo_CreateAndGetByLogin(t *testing.T) {
	// Arrange
	db := openIntegrationDB(t)
	repo := NewUserRepo(db)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	u := model.User{
		ID:           uuid.NewString(),
		Login:        "alice",
		PasswordHash: "hash",
		RegisteredAt: time.Now().UTC().Truncate(time.Second),
	}

	// Act
	got, err := repo.Create(ctx, u)
	selected, selectErr := repo.GetByLogin(ctx, u.Login)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, u, got)
	require.NoError(t, selectErr)
	assert.Equal(t, u.ID, selected.ID)
	assert.Equal(t, u.Login, selected.Login)
	assert.Equal(t, u.PasswordHash, selected.PasswordHash)
}

// TestOrderRepo_Create_ListByUser_ApplyAccrualResult проверяет базовый сценарий: создание заказа, получение списка
// заказов пользователя и применение результата начисления.
func TestOrderRepo_Create_ListByUser_ApplyAccrualResult(t *testing.T) {
	// Arrange
	db := openIntegrationDB(t)

	userRepo := NewUserRepo(db)
	orderRepo, err := NewOrderRepo(db, 2*time.Second)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	u := model.User{
		ID:           uuid.NewString(),
		Login:        "bob",
		PasswordHash: "hash",
		RegisteredAt: time.Now().UTC().Truncate(time.Second),
	}
	_, err = userRepo.Create(ctx, u)
	require.NoError(t, err)

	o := model.Order{
		Number:     "12345678903",
		Status:     model.OrderStatusNew,
		UploadedAt: time.Now().UTC().Truncate(time.Second),
	}

	// Act
	createErr := orderRepo.Create(ctx, u.ID, o)
	list, listErr := orderRepo.ListByUserID(ctx, u.ID)

	accrual := uint64(42)
	applyErr := orderRepo.ApplyAccrualResult(ctx, o.Number, model.OrderStatusProcessed, &accrual)
	list2, list2Err := orderRepo.ListByUserID(ctx, u.ID)

	// Assert
	require.NoError(t, createErr)
	require.NoError(t, listErr)
	assert.Len(t, list, 1)
	assert.Equal(t, o.Number, list[0].Number)
	assert.Equal(t, model.OrderStatusNew, list[0].Status)

	require.NoError(t, applyErr)
	require.NoError(t, list2Err)
	assert.Len(t, list2, 1)
	assert.Equal(t, model.OrderStatusProcessed, list2[0].Status)
	assert.NotNil(t, list2[0].Accrual)
	assert.Equal(t, uint64(42), *list2[0].Accrual)
}

// TestBalanceRepo_GetBalance_AndWithdrawals проверяет расчет агрегированного баланса пользователя и корректное
// сохранение/чтение операций списания.
func TestBalanceRepo_GetBalance_AndWithdrawals(t *testing.T) {
	// Arrange
	db := openIntegrationDB(t)

	userRepo := NewUserRepo(db)
	orderRepo, err := NewOrderRepo(db, 2*time.Second)
	require.NoError(t, err)
	balanceRepo := NewBalanceRepo(db)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	u := model.User{
		ID:           uuid.NewString(),
		Login:        "carol",
		PasswordHash: "hash",
		RegisteredAt: time.Now().UTC().Truncate(time.Second),
	}
	_, err = userRepo.Create(ctx, u)
	require.NoError(t, err)

	order := model.Order{Number: "55555555555", Status: model.OrderStatusNew, UploadedAt: time.Now().UTC()}
	require.NoError(t, orderRepo.Create(ctx, u.ID, order))

	accrual := uint64(100)
	require.NoError(t, orderRepo.ApplyAccrualResult(ctx, order.Number, model.OrderStatusProcessed, &accrual))

	// Act
	b, bErr := balanceRepo.GetBalance(ctx, u.ID)
	withdrawErr := balanceRepo.CreateWithdrawal(ctx, u.ID, "w1", 30, time.Now().UTC())
	b2, b2Err := balanceRepo.GetBalance(ctx, u.ID)
	ws, wsErr := balanceRepo.ListWithdrawals(ctx, u.ID)

	// Assert
	require.NoError(t, bErr)
	assert.Equal(t, uint64(100), b.Accrual)
	assert.Equal(t, uint64(0), b.Withdrawn)

	require.NoError(t, withdrawErr)
	require.NoError(t, b2Err)
	assert.Equal(t, uint64(100), b2.Accrual)
	assert.Equal(t, uint64(30), b2.Withdrawn)

	require.NoError(t, wsErr)
	require.Len(t, ws, 1)
	assert.Equal(t, "w1", ws[0].OrderNumber)
	assert.Equal(t, uint64(30), ws[0].Sum)
}
