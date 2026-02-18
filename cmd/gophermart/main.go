// Package main содержит точку входа сервиса "Гофермарт".
package main

import (
	"context"
	"fmt"
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
		_, _ = fmt.Fprintf(os.Stdout, "config error: %v\n", err)
		os.Exit(1)
	}

	logger, err := newLogger(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "logger init error: %v\n", err)
		os.Exit(1)
	}
	slog.SetDefault(logger)

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("app init error", slog.Any("err", err))
		os.Exit(1)
	}
	defer application.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err = application.Run(ctx); err != nil {
		slog.Error("service terminated with error", slog.Any("err", err))
		os.Exit(1)
	}

	slog.Info("service stopped (graceful)")
}
