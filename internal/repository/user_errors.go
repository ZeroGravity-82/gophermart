package repository

import "errors"

// ErrUserNotFound возвращается репозиторием, когда пользователь не найден.
var ErrUserNotFound = errors.New("user not found")
