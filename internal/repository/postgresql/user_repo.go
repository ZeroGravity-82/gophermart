package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"zerogravity-82/gophermart/internal/model"
	"zerogravity-82/gophermart/internal/repository"
)

// UserRepo реализует доступ к пользователям, хранящимся в PostgreSQL.
type UserRepo struct {
	db *sqlx.DB
}

// NewUserRepo создает UserRepo на основе подключения к БД.
func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create сохраняет нового пользователя в БД.
func (r *UserRepo) Create(ctx context.Context, u model.User) (model.User, error) {
	const q = `INSERT INTO "user" (id, login, password_hash, registered_at) VALUES ($1, $2, $3, $4)`
	_, err := r.db.ExecContext(ctx, q, u.ID, u.Login, u.PasswordHash, u.RegisteredAt)
	if err != nil {
		if isUniqueViolation(err) {
			return model.User{}, model.ErrLoginAlreadyTaken
		}
		return model.User{}, fmt.Errorf("failed to persist new user: %w", err)
	}
	return u, nil
}

// GetByLogin возвращает пользователя по логину.
//
// Если пользователь не найден, возвращает repository.ErrUserNotFound.
func (r *UserRepo) GetByLogin(ctx context.Context, login string) (model.User, error) {
	const q = `SELECT id, login, password_hash, registered_at FROM "user" WHERE login = $1`
	var u model.User
	if err := r.db.GetContext(ctx, &u, q, login); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, repository.ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("failed to select user by login: %w", err)
	}
	return u, nil
}
