package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var (
	// ErrInvalidToken возвращается, когда JWT не удается распарсить или провалидировать.
	ErrInvalidToken = errors.New("invalid token")
)

// JWTManager выпускает и валидирует access JWT.
//
// Он использует подпись HMAC-SHA256 (HS256) с заданным секретом.
type JWTManager struct {
	secret         []byte
	accessTokenTTL time.Duration
}

// NewJWTManager создает JWTManager.
//
// secret используется для подписи/проверки токенов. accessTokenTTL задает время жизни access-токена.
// logger используется для диагностических логов; если передан нулевой логгер, логирование отключается.
func NewJWTManager(secret string, accessTokenTTL time.Duration) (*JWTManager, error) {
	if secret == "" {
		return nil, errors.New("JWT secret is empty")
	}
	if accessTokenTTL <= 0 {
		return nil, errors.New("access token TTL must be positive")
	}
	return &JWTManager{secret: []byte(secret), accessTokenTTL: accessTokenTTL}, nil
}

// AccessClaims содержит стандартные зарегистрированные утверждения JWT.
//
// Subject (sub) используется как идентификатор пользователя.
type AccessClaims struct {
	jwt.RegisteredClaims
}

// IssueAccessToken создает и подписывает новый access-токен для указанного userID.
//
// Возвращаемое значение — компактная строка JWT, подходящая для заголовка `Authorization: Bearer <token>`.
func (m *JWTManager) IssueAccessToken(userID string) (string, error) {
	now := time.Now().UTC()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTokenTTL)),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := t.SignedString(m.secret)
	if err != nil {
		slog.Error("failed to sign access token", slog.String("user_id", userID), slog.Any("err", err))
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}
	slog.Debug(
		"access token issued",
		slog.String("user_id", userID),
		slog.Time("iat", claims.IssuedAt.Time),
		slog.Time("exp", claims.ExpiresAt.Time),
	)
	return tokenString, err
}

// ParseAccessToken парсит и валидирует строку токена, после чего возвращает распарсенные утверждения.
//
// В случае любых ошибок парсинга/валидации/подписи возвращает ErrInvalidToken.
func (m *JWTManager) ParseAccessToken(tokenString string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			slog.Warn("unexpected access token signing method", slog.Any("alg", token.Header["alg"]))
			return nil, fmt.Errorf("unexpected access token signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// GenerateRefreshToken возвращает криптографически стойкий непрозрачный refresh-токен.
//
// Токен предполагается хранить на стороне клиента и обменивать на новый access-токен. Его необходимо считать секретом.
func (m *JWTManager) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to generate refresh token", slog.Any("err", err))
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	slog.Debug("refresh token generated")

	// URL-safe without padding.
	return base64.RawURLEncoding.EncodeToString(b), nil
}
