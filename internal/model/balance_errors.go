package model

import "errors"

var (
	// ErrInsufficientFunds возвращается при попытке списания, когда на балансе недостаточно средств.
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrWithdrawConflict возвращается при повторной попытке списать средства по тому же номеру заказа.
	ErrWithdrawConflict = errors.New("funds have already been withdraw to pay for this order")
)
