package model

import "time"

// User описывает пользователя сервиса.
//
// Password используется только для входных данных (регистрация/логин) и не хранится в БД.
type User struct {
	ID           string    `db:"id"`
	Login        string    `db:"login"`
	Password     string    `db:"-"`
	PasswordHash string    `db:"password_hash"`
	RegisteredAt time.Time `db:"registered_at"`
}
