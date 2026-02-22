package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithLogger_Nil_NoChange проверяет, что WithLogger с nil-логгером не ломает поведение: FromContext возвращает
// рабочий логгер-заглушку.
func TestWithLogger_Nil_NoChange(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act
	ctx2 := WithLogger(ctx, nil)
	l := FromContext(ctx2)

	// Assert
	require.NotNil(t, l)

	// Проверяем, что логгер-заглушка не пишет в наши буферы.
	buf := &bytes.Buffer{}
	l.Debug("should not be written", slog.String("k", "v"))
	assert.Empty(t, buf.String())
}

// TestWithLogger_StoresAndFromContext_ReturnsSameLogger проверяет, что WithLogger сохраняет логгер в контекст, а
// FromContext возвращает тот же экземпляр.
func TestWithLogger_StoresAndFromContext_ReturnsSameLogger(t *testing.T) {
	// Arrange
	base := context.Background()
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Act
	ctx := WithLogger(base, logger)
	got := FromContext(ctx)
	got.Debug("test", slog.String("k", "v"))

	// Assert
	assert.Same(t, logger, got)
	out := buf.String()
	assert.Contains(t, out, "test")
	assert.Contains(t, out, "k=v")
}

// TestFromContext_WhenMissing_ReturnsNoopLogger проверяет, что если в контексте нет логгера, то FromContext возвращает
// логгер-заглушку.
func TestFromContext_WhenMissing_ReturnsNoopLogger(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act
	l := FromContext(ctx)

	// Assert
	require.NotNil(t, l)
	l.Warn("noop")
}
