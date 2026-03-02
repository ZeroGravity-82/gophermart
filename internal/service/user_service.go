package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"zerogravity-82/gophermart/internal/auth"
	"zerogravity-82/gophermart/internal/model"
	"zerogravity-82/gophermart/internal/repository"
)

// UserService реализует бизнес-логику регистрации и аутентификации пользователей.
//
// Сервис выпускает access JWT и управляет ротацией refresh-токенов.

type userRepository interface {
	Create(ctx context.Context, u model.User) (model.User, error)
	GetByLogin(ctx context.Context, login string) (model.User, error)
}

type refreshTokenRepository interface {
	Create(ctx context.Context, t model.RefreshToken) error
	FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (model.RefreshToken, error)
	Revoke(ctx context.Context, id string, revokedAt time.Time) error
}

type UserService struct {
	userRepo         userRepository
	refreshTokenRepo refreshTokenRepository
	jwtm             *auth.JWTManager
	refreshTokenTTL  time.Duration
}

// NewUserService создает UserService и настраивает значения по умолчанию.
//
// По умолчанию refresh-токен живет 30 суток.
func NewUserService(
	userRepo userRepository,
	refreshTokenRepo refreshTokenRepository,
	jwtm *auth.JWTManager,
) *UserService {
	return &UserService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		jwtm:             jwtm,
		refreshTokenTTL:  30 * 24 * time.Hour,
	}
}

// Register регистрирует пользователя и возвращает пару токенов (access/refresh).
func (us *UserService) Register(ctx context.Context, login, password string) (model.Tokens, error) {
	// Убеждаемся, что логин уникален
	_, err := us.userRepo.GetByLogin(ctx, login)
	if err == nil {
		return model.Tokens{}, model.ErrLoginAlreadyTaken
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		err = fmt.Errorf("failed to verify login uniqueness: %w", err)
		return model.Tokens{}, err
	}

	h, err := auth.HashPassword(password)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("failed to hash password for new user: %w", err)
	}

	uuidV7, err := uuid.NewV7()
	if err != nil {
		return model.Tokens{}, fmt.Errorf("failed to generate ID for new user: %w", err)
	}
	u := model.User{
		ID:           uuidV7.String(),
		Login:        login,
		PasswordHash: h,
		RegisteredAt: time.Now().UTC(),
	}
	_, err = us.userRepo.Create(ctx, u)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("failed to register new user: %w", err)
	}

	tokens, err := us.issueTokens(ctx, u.ID)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("failed to issue tokens for new user: %w", err)
	}
	return tokens, nil
}

// Login аутентифицирует пользователя и возвращает новую пару токенов (access/refresh).
func (us *UserService) Login(ctx context.Context, login, password string) (model.Tokens, error) {
	u, err := us.userRepo.GetByLogin(ctx, login)
	if err != nil {
		return model.Tokens{}, model.ErrAuthenticationFailed
	}
	if err := auth.CheckPasswordHash(password, u.PasswordHash); err != nil {
		return model.Tokens{}, model.ErrAuthenticationFailed
	}
	tokens, err := us.issueTokens(ctx, u.ID)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("failed to login user: %w", err)
	}
	return tokens, nil
}

// Refresh проверяет refresh-токен, отзывает его (ротация) и выдает новую пару токенов.
func (us *UserService) Refresh(ctx context.Context, refreshToken string) (model.Tokens, error) {
	if refreshToken == "" {
		return model.Tokens{}, model.ErrAuthenticationFailed
	}

	hash := sha256Hex(refreshToken)
	stored, err := us.refreshTokenRepo.FindActiveByHash(ctx, hash, time.Now().UTC())
	if err != nil {
		return model.Tokens{}, model.ErrAuthenticationFailed
	}

	// Ротация refresh-токена
	now := time.Now().UTC()
	if err := us.refreshTokenRepo.Revoke(ctx, stored.ID, now); err != nil {
		return model.Tokens{}, err
	}

	tokens, err := us.issueTokens(ctx, stored.UserID)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("failed to refresh token: %w", err)
	}
	return tokens, nil
}

// Logout отзывает refresh-токен пользователя, делая его недействительным.
func (us *UserService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return model.ErrAuthenticationFailed
	}
	hash := sha256Hex(refreshToken)
	stored, err := us.refreshTokenRepo.FindActiveByHash(ctx, hash, time.Now().UTC())
	if err != nil {
		return model.ErrAuthenticationFailed
	}
	return us.refreshTokenRepo.Revoke(ctx, stored.ID, time.Now().UTC())
}

func (us *UserService) issueTokens(ctx context.Context, userID string) (model.Tokens, error) {
	access, err := us.jwtm.IssueAccessToken(ctx, userID)
	if err != nil {
		return model.Tokens{}, err
	}
	refresh, err := us.jwtm.GenerateRefreshToken(ctx)
	if err != nil {
		return model.Tokens{}, err
	}

	uuidV7, err := uuid.NewV7()
	if err != nil {
		return model.Tokens{}, fmt.Errorf("failed to generate ID for refresh token: %w", err)
	}
	now := time.Now().UTC()
	rt := model.RefreshToken{
		ID:        uuidV7.String(),
		UserID:    userID,
		TokenHash: sha256Hex(refresh),
		ExpiresAt: now.Add(us.refreshTokenTTL),
		IssuedAt:  now,
		RevokedAt: nil,
	}
	if err := us.refreshTokenRepo.Create(ctx, rt); err != nil {
		return model.Tokens{}, fmt.Errorf("failed to persist refresh token: %w", err)
	}

	return model.Tokens{AccessToken: access, RefreshToken: refresh}, nil
}

func sha256Hex(v string) string {
	s := sha256.Sum256([]byte(v))
	return hex.EncodeToString(s[:])
}
