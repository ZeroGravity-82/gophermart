package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/model"
)

type orderRepoStub struct {
	err    error
	orders []model.Order
}

func (s orderRepoStub) Create(_ context.Context, _ string, _ model.Order) error {
	return s.err
}

func (s orderRepoStub) ListByUserID(_ context.Context, _ string) ([]model.Order, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.orders, nil
}

// TestOrderService_Upload проверяет валидацию номера заказа и маппинг ошибок репозитория.
func TestOrderService_Upload(t *testing.T) {
	// Arrange
	tests := []struct {
		name string

		number  string
		repoErr error

		wantErr error
	}{
		{
			name:   "ok",
			number: "79927398713",
		},
		{
			name:    "invalid luhn",
			number:  "79927398714", // invalid checksum
			wantErr: model.ErrInvalidOrderNumber,
		},
		{
			name:    "same user duplicate maps",
			number:  "79927398713",
			repoErr: model.ErrOrderAlreadyExistsSameUser,
			wantErr: model.ErrOrderAlreadyExistsSameUser,
		},
		{
			name:    "other user duplicate maps",
			number:  "79927398713",
			repoErr: model.ErrOrderAlreadyExistsOtherUser,
			wantErr: model.ErrOrderAlreadyExistsOtherUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			s := NewOrderService(orderRepoStub{err: tt.repoErr})

			// Act
			err := s.Upload(context.Background(), "user-1", tt.number)

			// Assert
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestOrderService_List проверяет получение списка заказов пользователя.
func TestOrderService_List(t *testing.T) {
	// Arrange
	svc := NewOrderService(orderRepoStub{orders: []model.Order{{Number: "1"}, {Number: "2"}}})

	// Act
	orders, err := svc.List(context.Background(), "user-1")

	// Assert
	require.NoError(t, err)
	assert.Len(t, orders, 2)
}
