package repository

import "errors"

// ErrRefreshTokenNotFound возвращается репозиторием, когда не найден активный (не отозванный и не истекший)
// refresh-токен.
var ErrRefreshTokenNotFound = errors.New("refresh token not found")
