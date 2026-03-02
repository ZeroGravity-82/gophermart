package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHashPassword_Empty проверяет ошибку при попытке хэшировать пустой пароль.
func TestHashPassword_Empty(t *testing.T) {
	// Arrange
	password := ""

	// Act
	_, err := HashPassword(password)

	// Assert
	require.Error(t, err)
}

// TestHashPassword проверяет генерацию хэша пароля.
func TestHashPassword(t *testing.T) {
	// Arrange
	tests := []struct {
		name      string
		password  string
		wantError bool
	}{
		{
			name:      "fail with empty password",
			password:  "",
			wantError: true,
		},
		{
			name:      "can hash non-empty password",
			password:  "qwerty",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			h, err := HashPassword(tt.password)

			// Assert
			if tt.wantError {
				require.Error(t, err)
				assert.Empty(t, h)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, h)
			}
		})
	}
}

// TestCheckPasswordHash проверяет валидацию пароля по сохраненному хэшу.
func TestCheckPasswordHash(t *testing.T) {
	// Arrange
	validPassword := "qwerty"
	wrongPassword := "wrong"
	h, err := HashPassword(validPassword)
	require.NoError(t, err)

	tests := []struct {
		name      string
		password  string
		wantError bool
	}{
		{
			name:      "can check valid password",
			password:  validPassword,
			wantError: false,
		},
		{
			name:      "can check wrong password",
			password:  wrongPassword,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err = CheckPasswordHash(tt.password, h)

			// Assert
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
