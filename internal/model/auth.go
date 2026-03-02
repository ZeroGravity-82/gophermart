package model

import "time"

// Tokens представляет пару токенов (access/refresh), возвращаемую клиенту.
type Tokens struct {
	AccessToken  string
	RefreshToken string
}

// RefreshToken описывает запись refresh-токена в хранилище.
//
// TokenHash хранится в БД вместо исходного токена.
type RefreshToken struct {
	ID        string     `db:"id"`
	UserID    string     `db:"user_id"`
	TokenHash string     `db:"token_hash"`
	ExpiresAt time.Time  `db:"expires_at"`
	IssuedAt  time.Time  `db:"issued_at"`
	RevokedAt *time.Time `db:"revoked_at"`
}
