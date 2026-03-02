// Package main содержит точку входа сервиса "Гофермарт".
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"zerogravity-82/gophermart/internal/app"
	"zerogravity-82/gophermart/internal/config"
)

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		// логгер еще не сконфигурирован
		log.Fatalf("config error: %v", err)
	}

	logger, err := newLogger(cfg.Logging)
	if err != nil {
		// логгер еще не сконфигурирован
		log.Fatalf("logger config error: %v", err)
	}

	if err := run(cfg, logger); err != nil {
		logger.Error("service terminated with error", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("service stopped (graceful)")
}

func run(cfg config.Config, logger *slog.Logger) error {
	application, err := app.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("app init error: %w", err)
	}
	defer application.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return application.Run(ctx)
}
