package service

import (
	"context"
	"fmt"
	"time"

	"zerogravity-82/gophermart/internal/model"
)

// OrderService содержит бизнес-логику работы с заказами.
type OrderService struct {
	orderRepo orderRepository
}

type orderRepository interface {
	Create(ctx context.Context, userID string, o model.Order) error
	ListByUserID(ctx context.Context, userID string) ([]model.Order, error)
}

// NewOrderService создает OrderService.
func NewOrderService(orderRepo orderRepository) *OrderService {
	return &OrderService{orderRepo: orderRepo}
}

// Upload принимает номер заказа, валидирует его и сохраняет заказ.
func (s *OrderService) Upload(ctx context.Context, userID, number string) error {
	if !validLuhn(number) {
		return model.ErrInvalidOrderNumber
	}

	err := s.orderRepo.Create(ctx, userID, model.Order{
		Number:     number,
		Status:     model.OrderStatusNew,
		UploadedAt: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("failed to upload order: %w", err)
	}
	return nil
}

// validLuhn определяет, проходит ли строка s проверку алгоритма Луны.
// Строка s должна представлять собой последовательность цифр.
func validLuhn(s string) bool {
	if s == "" {
		return false
	}
	sum := 0
	alt := false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		n := int(c - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

// List возвращает список загруженных номеров заказов для указанного пользователя. Список отсортирован по времени
// загрузки (от самых новых к самым старым).
func (s *OrderService) List(ctx context.Context, userID string) ([]model.Order, error) {
	orders, err := s.orderRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	return orders, nil
}
