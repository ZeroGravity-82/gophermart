// Package app отвечает за сборку зависимостей сервиса, а также запуск HTTP-сервера и воркера работы с сервисом расчета
// начислений баллов лояльности.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"golang.org/x/sync/errgroup"

	"zerogravity-82/gophermart/internal/accrual"
	"zerogravity-82/gophermart/internal/auth"
	"zerogravity-82/gophermart/internal/config"
	"zerogravity-82/gophermart/internal/httpserver"
	"zerogravity-82/gophermart/internal/logging"
	"zerogravity-82/gophermart/internal/repository/postgresql"
	"zerogravity-82/gophermart/internal/service"
)

// App инициализирует зависимости сервиса, также запускает HTTP-сервер и воркер работы с сервисом расчета начислений
// баллов лояльности.
type App struct {
	db     *sqlx.DB
	srv    *httpserver.HTTPServer
	worker *service.AccrualWorker
	logger *slog.Logger
}

// New создает App - подключается к БД, применяет миграции и настраивает прикладные сервисы.
func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = logging.NopLogger()
	}

	db, err := sqlx.Connect("pgx", cfg.DatabaseURI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %w", err)
	}
	if err = applyMigrations(db); err != nil {
		return nil, fmt.Errorf("migrations error: %w", err)
	}

	userRepo := postgresql.NewUserRepo(db)
	refreshTokenRepo := postgresql.NewRefreshTokenRepo(db)
	jwtm, err := auth.NewJWTManager(cfg.JWTSecret, 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("JWT manager init error: %w", err)
	}
	us := service.NewUserService(userRepo, refreshTokenRepo, jwtm)

	orderRepo, err := postgresql.NewOrderRepo(db, cfg.AccrualLockTTL)
	if err != nil {
		return nil, fmt.Errorf("order repository init error: %w", err)
	}
	os := service.NewOrderService(orderRepo)

	balanceRepo := postgresql.NewBalanceRepo(db)
	bs := service.NewBalanceService(balanceRepo)

	srv := httpserver.New(cfg.RunAddr, jwtm, us, os, bs)

	accrualClient, err := accrual.New(cfg.AccrualAddr)
	if err != nil {
		return nil, fmt.Errorf("accrual client init error: %w", err)
	}
	worker, err := service.NewAccrualWorker(
		orderRepo,
		accrualClient,
		cfg.AccrualPollInterval,
		cfg.AccrualBatchSize,
		cfg.AccrualWorkers,
	)
	if err != nil {
		return nil, fmt.Errorf("worker init error: %w", err)
	}

	return &App{
		db:     db,
		srv:    srv,
		worker: worker,
		logger: logger,
	}, nil
}

func applyMigrations(db *sqlx.DB) error {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to initialize database driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	return nil
}

// Run запускает HTTP-сервер и воркер работы с сервисом расчета начислений баллов лояльности.
// Блокируется до остановки по сигналу завершения или из-за ошибки HTTP-сервера.
func (a *App) Run(ctx context.Context) error {
	ctx = logging.WithLogger(ctx, a.logger)

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error { return a.srv.Run(ctx) })
	eg.Go(func() error { a.worker.Run(ctx); return nil })
	return eg.Wait()
}

// Close закрывает ресурсы приложения (например, соединение с БД).
func (a *App) Close() {
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.logger.Error("failed to close db", slog.Any("err", err))
		}
	}
}
