package model

import "time"

// Withdrawal описывает факт списания баллов с накопительного счета лояльности пользователя.
// Sum хранится в минорных единицах баллов лояльности.
type Withdrawal struct {
	OrderNumber string    `db:"order_number"`
	Sum         uint64    `db:"sum"`
	ProcessedAt time.Time `db:"processed_at"`
}
