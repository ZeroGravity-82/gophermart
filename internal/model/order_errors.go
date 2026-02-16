package model

import "errors"

var (
	// ErrInvalidOrderNumber возвращается при некорректном номере заказа.
	ErrInvalidOrderNumber = errors.New("invalid order number")
	// ErrOrderAlreadyExistsSameUser возвращается при повторной загрузке одного и того же заказа тем же пользователем.
	ErrOrderAlreadyExistsSameUser = errors.New("order already uploaded by this user")
	// ErrOrderAlreadyExistsOtherUser возвращается, когда заказ уже загружен другим пользователем.
	ErrOrderAlreadyExistsOtherUser = errors.New("order already uploaded by another user")
)
