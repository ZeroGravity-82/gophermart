package postgresql

import (
	"errors"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
)

// isUniqueViolation сообщает, что err соответствует нарушению уникальности в PostgreSQL (SQLSTATE 23505).
//
// При использовании database/sql (и оберток вроде sqlx) ошибка драйвера может быть обернута так,
// что исходный тип *pgconn.PgError становится недоступен для errors.As. Сначала пытаемся определить
// ошибку по типу; если не получилось — используем запасной вариант и ищем SQLSTATE 23505 в тексте.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgerrcode.UniqueViolation
	}

	return strings.Contains(err.Error(), "SQLSTATE 23505")
}
