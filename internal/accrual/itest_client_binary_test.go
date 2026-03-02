//go:build integration

package accrual

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_GetAccrual_WithRealBinary поднимает реальный accrual-сервис из `cmd/accrual` (предварительно собранный
// бинарник) и проверяет, что HTTP-клиент умеет обрабатывать ответ 204 (заказ еще не зарегистрирован).
func TestClient_GetAccrual_WithRealBinary(t *testing.T) {
	// Arrange
	dsn := os.Getenv("TEST_DATABASE_URI")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("TEST_DATABASE_URI is not set; skipping integration test")
	}

	binPath, err := accrualBinaryPath()
	require.NoError(t, err)

	addr := freeTCPAddr(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, binPath, "-a", addr, "-d", dsn)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	baseURL := "http://" + addr
	waitForAccrualAPI(t, baseURL, 5*time.Second)

	client, err := New(baseURL)
	require.NoError(t, err)

	// Act
	_, err = client.GetAccrual(ctx, "12345678903")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOrderNotProcessed)
}

func accrualBinaryPath() (string, error) {
	// Pick binary by OS/arch; for local Linux amd64 use provided file.
	suffix := ""
	switch runtime.GOOS {
	case "linux":
		suffix = "linux_amd64"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			suffix = "darwin_arm64"
		} else {
			suffix = "darwin_amd64"
		}
	case "windows":
		suffix = "windows_amd64"
	default:
		return "", fmt.Errorf("unsupported GOOS for accrual binary: %s", runtime.GOOS)
	}

	p := filepath.Join("..", "..", "cmd", "accrual", "accrual_"+suffix)
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(abs); statErr != nil {
		return "", fmt.Errorf("accrual binary not found at %s: %w", abs, statErr)
	}
	return abs, nil
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

func waitForAccrualAPI(t *testing.T, baseURL string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 250 * time.Millisecond}

	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/orders/0", nil)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()

			// Для готовности достаточно, чтобы ручка отвечала ожидаемым кодом.
			// 204 означает "заказ не зарегистрирован", что для пустой БД нормально.
			if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
				return
			}
		}

		// Ошибки сети/таймауты считаем нормальными во время прогрева.
		if errors.Is(err, context.DeadlineExceeded) {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("accrual service did not become ready at %s", baseURL)
}
