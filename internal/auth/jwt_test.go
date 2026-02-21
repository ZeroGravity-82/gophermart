package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewJWTManager проверяет создание JWTManager и валидацию входных параметров.
func TestNewJWTManager(t *testing.T) {
	// Arrange
	tests := []struct {
		name           string
		secret         string
		accessTokenTTL time.Duration
		wantJWTManager JWTManager
		wantErr        bool
	}{
		{
			name:           "fail with empty secret",
			secret:         "",
			accessTokenTTL: 15 * time.Minute,
			wantJWTManager: JWTManager{},
			wantErr:        true,
		},
		{
			name:           "fail with zero access token TTL",
			secret:         "secret",
			accessTokenTTL: 0,
			wantJWTManager: JWTManager{},
			wantErr:        true,
		},
		{
			name:           "fail with negative access token TTL",
			secret:         "secret",
			accessTokenTTL: -15 * time.Minute,
			wantJWTManager: JWTManager{},
			wantErr:        true,
		},
		{
			name:           "can create JWT manager",
			secret:         "secret",
			accessTokenTTL: 15 * time.Minute,
			wantJWTManager: JWTManager{
				secret:         []byte("secret"),
				accessTokenTTL: 15 * time.Minute,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			m, err := NewJWTManager(tt.secret, tt.accessTokenTTL)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, m)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantJWTManager, *m)
			}

		})
	}
}

// TestIssueAndParseAccessToken_OK проверяет выпуск access-токена и последующий разбор claims.
func TestIssueAndParseAccessToken_OK(t *testing.T) {
	// Arrange
	m, err := NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	// Act
	tok, err := m.IssueAccessToken(context.Background(), "user-123")
	require.NoError(t, err)
	assert.NotEmpty(t, tok)

	claims, err := m.ParseAccessToken(context.Background(), tok)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, "user-123", claims.Subject)
	assert.NotNil(t, claims.IssuedAt)
	assert.NotNil(t, claims.ExpiresAt)
	assert.True(t, claims.ExpiresAt.After(claims.IssuedAt.Time))
}

// TestParseAccessToken_FailWithInvalidToken проверяет ошибку при разборе некорректного токена.
func TestParseAccessToken_FailWithInvalidToken(t *testing.T) {
	// Arrange
	m, err := NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	// Act
	_, err = m.ParseAccessToken(context.Background(), "not-a-jwt")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

// TestParseAccessToken_FailWithWrongSecret проверяет ошибку, если токен подписан другим секретом.
func TestParseAccessToken_FailWithWrongSecret(t *testing.T) {
	// Arrange
	m1, err := NewJWTManager("secret-1", 15*time.Minute)
	require.NoError(t, err)
	m2, err := NewJWTManager("secret-2", 15*time.Minute)
	require.NoError(t, err)

	tok, err := m1.IssueAccessToken(context.Background(), "user-123")
	require.NoError(t, err)

	// Act
	_, err = m2.ParseAccessToken(context.Background(), tok)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

// TestParseAccessToken_FailWithExpiredToken проверяет ошибку при разборе истекшего токена.
func TestParseAccessToken_FailWithExpiredToken(t *testing.T) {
	// Arrange
	m, err := NewJWTManager("secret", 1*time.Millisecond)
	require.NoError(t, err)

	tok, err := m.IssueAccessToken(context.Background(), "user-123")
	require.NoError(t, err)

	// Гарантируем, что токен истек
	time.Sleep(10 * time.Millisecond)

	// Act
	_, err = m.ParseAccessToken(context.Background(), tok)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

// TestParseAccessToken_FailWithRejectedNonHMACAlg проверяет отклонение токена с неподдерживаемым алгоритмом подписи.
func TestParseAccessToken_FailWithRejectedNonHMACAlg(t *testing.T) {
	// Arrange
	m, err := NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	claims := AccessClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: "user-123"}}
	// Подписываем методом "none", чтобы гарантировать, что ParseWithClaims увидит не HMAC-алгоритм.
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tok, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	// Act
	_, err = m.ParseAccessToken(context.Background(), tok)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

// TestGenerateRefreshToken проверяет генерацию refresh-токена.
func TestGenerateRefreshToken(t *testing.T) {
	// Arrange
	m, err := NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	// Act
	t1, err := m.GenerateRefreshToken(context.Background())
	require.NoError(t, err)
	t2, err := m.GenerateRefreshToken(context.Background())
	require.NoError(t, err)

	// Assert
	assert.NotEmpty(t, t1)
	assert.NotEmpty(t, t2)
	assert.NotEqual(t, t1, t2)
}
