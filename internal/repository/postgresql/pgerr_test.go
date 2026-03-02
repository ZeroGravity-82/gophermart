package postgresql

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/stretchr/testify/assert"
)

type unwrapErr struct{ err error }

func (u unwrapErr) Error() string { return "wrapped: " + u.err.Error() }
func (u unwrapErr) Unwrap() error { return u.err }

// Test_isUniqueViolation проверяет распознавание нарушения уникальности (SQLSTATE 23505).
func Test_isUniqueViolation(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "typed unique violation", err: &pgconn.PgError{Code: pgerrcode.UniqueViolation}, want: true},
		{name: "typed other pg error", err: &pgconn.PgError{Code: pgerrcode.ForeignKeyViolation}, want: false},
		{
			name: "wrapped typed pg error",
			err:  fmt.Errorf("outer: %w", &pgconn.PgError{Code: pgerrcode.UniqueViolation}),
			want: true,
		},
		{
			name: "custom unwrap chain",
			err:  unwrapErr{err: &pgconn.PgError{Code: pgerrcode.UniqueViolation}},
			want: true,
		},
		{
			name: "fallback by text",
			err:  errors.New("ERROR: duplicate key value violates unique constraint \"order_pkey\" (SQLSTATE 23505)"),
			want: true,
		},
		{name: "other error text", err: errors.New("some error"), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			err := tt.err

			// Act
			got := isUniqueViolation(err)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
