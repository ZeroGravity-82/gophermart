package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"zerogravity-82/gophermart/internal/config"
)

func newLogger(cfg config.Config) (*slog.Logger, error) {
	level, err := parseLogLevel(cfg.LogLevel)
	if err != nil {
		return nil, err
	}

	var w io.Writer = os.Stdout
	opts := &slog.HandlerOptions{Level: level}

	switch strings.ToLower(cfg.LogFormat) {
	case "json", "":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	case "text":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("unsupported log format: %s", cfg.LogFormat)
	}
}

func parseLogLevel(v string) (slog.Level, error) {
	switch strings.ToLower(v) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, errors.New("unsupported log level: " + v)
	}
}
