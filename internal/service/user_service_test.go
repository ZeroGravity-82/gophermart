package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/auth"
	"zerogravity-82/gophermart/internal/model"
	"zerogravity-82/gophermart/internal/repository"
)

type userRepoStub struct {
	byLogin map[string]model.User
	getErr  error

	createErr error
	created   []model.User
}

func (s *userRepoStub) Create(_ context.Context, u model.User) (model.User, error) {
	if s.createErr != nil {
		return model.User{}, s.createErr
	}
	s.created = append(s.created, u)
	if s.byLogin != nil {
		s.byLogin[u.Login] = u
	}
	return u, nil
}

func (s *userRepoStub) GetByLogin(_ context.Context, login string) (model.User, error) {
	if s.getErr != nil {
		return model.User{}, s.getErr
	}
	if s.byLogin == nil {
		return model.User{}, repository.ErrUserNotFound
	}
	u, ok := s.byLogin[login]
	if !ok {
		return model.User{}, repository.ErrUserNotFound
	}
	return u, nil
}

type refreshTokenRepoStub struct {
	storedByHash map[string]model.RefreshToken

	createErr error
	revokeErr error

	revoked []string
}

func (s *refreshTokenRepoStub) Create(_ context.Context, t model.RefreshToken) error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.storedByHash == nil {
		s.storedByHash = make(map[string]model.RefreshToken)
	}
	s.storedByHash[t.TokenHash] = t
	return nil
}

func (s *refreshTokenRepoStub) FindActiveByHash(
	_ context.Context,
	tokenHash string,
	now time.Time,
) (model.RefreshToken, error) {
	t, ok := s.storedByHash[tokenHash]
	if !ok {
		return model.RefreshToken{}, repository.ErrRefreshTokenNotFound
	}
	if t.RevokedAt != nil {
		return model.RefreshToken{}, repository.ErrRefreshTokenNotFound
	}
	if !t.ExpiresAt.After(now) {
		return model.RefreshToken{}, repository.ErrRefreshTokenNotFound
	}
	return t, nil
}

func (s *refreshTokenRepoStub) Revoke(_ context.Context, id string, revokedAt time.Time) error {
	if s.revokeErr != nil {
		return s.revokeErr
	}
	for hash, t := range s.storedByHash {
		if t.ID == id {
			t.RevokedAt = &revokedAt
			s.storedByHash[hash] = t
			break
		}
	}
	s.revoked = append(s.revoked, id)
	return nil
}

// TestUserService_Register_OK проверяет регистрацию нового пользователя.
func TestUserService_Register_OK(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	userRepo := &userRepoStub{byLogin: map[string]model.User{}}
	rtRepo := &refreshTokenRepoStub{storedByHash: map[string]model.RefreshToken{}}
	svc := NewUserService(userRepo, rtRepo, jwtm)

	// Act
	tokens, err := svc.Register(context.Background(), "login", "pass")

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	require.Len(t, userRepo.created, 1)
}

// TestUserService_Register_LoginTaken проверяет конфликт при регистрации с занятым логином.
func TestUserService_Register_LoginTaken(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	existingHash, err := auth.HashPassword("pass")
	require.NoError(t, err)

	userRepo := &userRepoStub{
		byLogin: map[string]model.User{"login": {ID: "u1", Login: "login", PasswordHash: existingHash}},
	}
	rtRepo := &refreshTokenRepoStub{storedByHash: map[string]model.RefreshToken{}}
	svc := NewUserService(userRepo, rtRepo, jwtm)

	// Act
	_, err = svc.Register(context.Background(), "login", "pass")

	// Assert
	assert.ErrorIs(t, err, model.ErrLoginAlreadyTaken)
}

// TestUserService_Login_OK проверяет успешный логин пользователя.
func TestUserService_Login_OK(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	h, err := auth.HashPassword("pass")
	require.NoError(t, err)

	userRepo := &userRepoStub{byLogin: map[string]model.User{"login": {ID: "u1", Login: "login", PasswordHash: h}}}
	rtRepo := &refreshTokenRepoStub{storedByHash: map[string]model.RefreshToken{}}
	svc := NewUserService(userRepo, rtRepo, jwtm)

	// Act
	tokens, err := svc.Login(context.Background(), "login", "pass")

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
}

// TestUserService_Refresh_OK проверяет ротацию refresh-токена.
func TestUserService_Refresh_OK(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	userRepo := &userRepoStub{byLogin: map[string]model.User{}}
	rtRepo := &refreshTokenRepoStub{storedByHash: map[string]model.RefreshToken{}}
	svc := NewUserService(userRepo, rtRepo, jwtm)

	// Сначала выпускаем токены, чтобы refresh-токен был сохранен в rtRepo.
	initial, err := svc.Register(context.Background(), "login", "pass")
	require.NoError(t, err)

	// Act
	updated, err := svc.Refresh(context.Background(), initial.RefreshToken)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, updated.AccessToken)
	assert.NotEmpty(t, updated.RefreshToken)
	assert.NotEqual(t, initial.RefreshToken, updated.RefreshToken)
	require.Len(t, rtRepo.revoked, 1)
}

// TestUserService_Refresh_EmptyToken проверяет ошибку при пустом refresh-токене.
func TestUserService_Refresh_EmptyToken(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	userRepo := &userRepoStub{byLogin: map[string]model.User{}}
	rtRepo := &refreshTokenRepoStub{storedByHash: map[string]model.RefreshToken{}}
	svc := NewUserService(userRepo, rtRepo, jwtm)

	// Act
	_, err = svc.Refresh(context.Background(), "")

	// Assert
	assert.ErrorIs(t, err, model.ErrAuthenticationFailed)
}

// TestUserService_Login_WrongPassword проверяет ошибку при неверном пароле.
func TestUserService_Login_WrongPassword(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	h, err := auth.HashPassword("pass")
	require.NoError(t, err)

	userRepo := &userRepoStub{byLogin: map[string]model.User{"login": {ID: "u1", Login: "login", PasswordHash: h}}}
	rtRepo := &refreshTokenRepoStub{storedByHash: map[string]model.RefreshToken{}}
	svc := NewUserService(userRepo, rtRepo, jwtm)

	// Act
	_, err = svc.Login(context.Background(), "login", "wrong")

	// Assert
	assert.ErrorIs(t, err, model.ErrAuthenticationFailed)
}

// TestUserService_Register_RepoError проверяет обработку ошибки репозитория при проверке уникальности.
func TestUserService_Register_RepoError(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	userRepo := &userRepoStub{getErr: errors.New("db down")}
	rtRepo := &refreshTokenRepoStub{storedByHash: map[string]model.RefreshToken{}}
	svc := NewUserService(userRepo, rtRepo, jwtm)

	// Act
	_, err = svc.Register(context.Background(), "login", "pass")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to verify login uniqueness")
}
