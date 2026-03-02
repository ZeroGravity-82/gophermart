//go:build integration

package postgresql

// Файл содержит вспомогательные функции для интеграционных тестов слоя postgresql-репозиториев.
//
// Тесты выполняются только при указании build-тега `integration` и при наличии переменной окружения TEST_DATABASE_URI.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	integrationDBURLEnv = "TEST_DATABASE_URI"
)

// openIntegrationDB открывает подключение к PostgreSQL для интеграционных тестов.
//
// Поведение:
//   - Если переменная окружения TEST_DATABASE_URI не задана, тест пропускается.
//   - Автоматически применяются миграции из `migrations/`.
//   - Таблицы очищаются (TRUNCATE), чтобы каждый тестовый запуск начинался с чистого состояния.
func openIntegrationDB(t *testing.T) *sqlx.DB {
	t.Helper()

	dsn, ok := os.LookupEnv(integrationDBURLEnv)
	if !ok || dsn == "" {
		t.Skipf("%s is not set; skipping integration test", integrationDBURLEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		t.Fatalf("failed to connect to integration DB: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := applyMigrationsForTests(db); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	// Гарантирует чистое состояние перед запуском тестов.
	if err := truncateAllForTests(db); err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}

	return db
}

// applyMigrationsForTests накатывает миграции на тестовую БД.
//
// Важно: путь к миграциям указан относительно директории пакета.
func applyMigrationsForTests(db *sqlx.DB) error {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("init migration driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://../../../migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// truncateAllForTests очищает данные во всех таблицах доменной модели.
//
// Используется для изоляции тестов (чтобы результаты не зависели от предыдущих прогонов).
func truncateAllForTests(db *sqlx.DB) error {
	// Порядок имеет значение из-за внешних ключей.
	_, err := db.Exec(`TRUNCATE TABLE withdrawal, "order", "user" RESTART IDENTITY CASCADE`)
	return err
}
