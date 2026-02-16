// Package main содержит точку входа сервиса "Гофермарт".
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"zerogravity-82/gophermart/internal/app"
	"zerogravity-82/gophermart/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	cfg, err := config.GetConfig()
	if err != nil {
		slog.Error("config error", slog.Any("err", err))
		os.Exit(1)
	}

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
