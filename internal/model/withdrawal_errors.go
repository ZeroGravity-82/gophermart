package model

import "errors"

var (
	// ErrInvalidWithdrawalSum возвращается при некорректной сумме списания.
	ErrInvalidWithdrawalSum = errors.New("invalid withdrawal sum")
)
