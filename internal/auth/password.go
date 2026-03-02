package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword вычисляет bcrypt-хэш для пароля.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("failed to hash empty password")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(b), nil
}

// CheckPasswordHash сравнивает пароль с сохраненным bcrypt-хэшем.
func CheckPasswordHash(password, passwordHash string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}
	return nil
}
