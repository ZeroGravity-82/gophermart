// Package dto содержит транспортные структуры (DTO) и преобразования между доменными моделями и транспортным
// представлением в формате HTTP/JSON.
package dto

import (
	"time"

	"zerogravity-82/gophermart/internal/model"
)

// OrderResponse — транспортное (HTTP/JSON) представление заказа.
// Accrual передается в баллах лояльности в вещественном представлении с 2 знаками после запятой.
type OrderResponse struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	UploadedAt time.Time `json:"uploaded_at"`
	Accrual    *float64  `json:"accrual,omitempty"`
}

// ToOrderResponse преобразует доменную модель заказа в DTO для передачи через HTTP в формате JSON.
func ToOrderResponse(o model.Order) OrderResponse {
	var accrual *float64
	if o.Accrual != nil {
		v := float64(*o.Accrual) / model.AccrualScale
		accrual = &v
	}
	return OrderResponse{
		Number:     o.Number,
		Status:     string(o.Status),
		UploadedAt: o.UploadedAt,
		Accrual:    accrual,
	}
}
