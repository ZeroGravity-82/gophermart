package dto

import (
	"time"

	"zerogravity-82/gophermart/internal/model"
)

// WithdrawalResponse — транспортное (HTTP/JSON) представление model.Withdrawal.
// Sum передается в мажорных единицах баллов лояльности (float64) с 2 знаками после запятой.
type WithdrawalResponse struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

// ToWithdrawalResponse преобразует доменную модель списания в DTO для ответа API.
func ToWithdrawalResponse(w model.Withdrawal) WithdrawalResponse {
	return WithdrawalResponse{
		Order:       w.OrderNumber,
		Sum:         float64(w.Sum) / model.AccrualScale,
		ProcessedAt: w.ProcessedAt,
	}
}
