package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"zerogravity-82/gophermart/internal/model"
	"zerogravity-82/gophermart/internal/repository"
)

// RefreshTokenRepo реализует доступ к refresh-токенам, хранящимся в PostgreSQL.
type RefreshTokenRepo struct {
	db *sqlx.DB
}

// NewRefreshTokenRepo создает RefreshTokenRepo на основе подключения к БД.
func NewRefreshTokenRepo(db *sqlx.DB) *RefreshTokenRepo {
	return &RefreshTokenRepo{db: db}
}

// Create сохраняет refresh-токен в БД.
func (r *RefreshTokenRepo) Create(ctx context.Context, t model.RefreshToken) error {
	const q = `INSERT INTO refresh_token (id, user_id, token_hash, expires_at, issued_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, q, t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.IssuedAt, t.RevokedAt)
	if err != nil {
		return fmt.Errorf("failed to persist new refresh token: %w", err)
	}
	return nil
}

// FindActiveByHash ищет активный (не отозванный и не истекший) refresh-токен по хэшу.
//
// Если токен не найден, возвращает repository.ErrRefreshTokenNotFound.
func (r *RefreshTokenRepo) FindActiveByHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (model.RefreshToken, error) {
	const q = `SELECT id, user_id, token_hash, expires_at, issued_at, revoked_at
		FROM refresh_token
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2`
	var t model.RefreshToken
	if err := r.db.GetContext(ctx, &t, q, tokenHash, now); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.RefreshToken{}, repository.ErrRefreshTokenNotFound
		}
		return model.RefreshToken{}, fmt.Errorf("select refresh token: %w", err)
	}
	return t, nil
}

// Revoke помечает refresh-токен как отозванный.
func (r *RefreshTokenRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	const q = `UPDATE refresh_token SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, id, revokedAt)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}
