package model

import (
	"time"
)

// OrderStatus представляет собой типизированный статус заказа.
type OrderStatus string

const (
	// OrderStatusNew — заказ загружен в сервис, но не попал в обработку.
	OrderStatusNew OrderStatus = "NEW"
	// OrderStatusProcessing — вознаграждение за заказ рассчитывается.
	OrderStatusProcessing OrderStatus = "PROCESSING"
	// OrderStatusInvalid — сервис расчета начислений баллов лояльности отказал в расчете.
	OrderStatusInvalid OrderStatus = "INVALID"
	// OrderStatusProcessed — данные по заказу проверены и информация о расчете успешно получена.
	OrderStatusProcessed OrderStatus = "PROCESSED"
)

// Order описывает заказ пользователя и количество начисленных за него баллов лояльности (если они уже рассчитаны).
type Order struct {
	Number     string      `db:"number"`
	Status     OrderStatus `db:"status"`
	UploadedAt time.Time   `db:"uploaded_at"`
	Accrual    *uint64     `db:"accrual"`
}
