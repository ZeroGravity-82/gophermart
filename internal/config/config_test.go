package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetConfig_Default проверяет поведение по умолчанию, когда ни флаги, ни переменные окружения сервиса не заданы.
func TestGetConfig_Default(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"cmd"}
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "127.0.0.1:8081")
	t.Setenv("DATABASE_URI", "postgres://user:pass@localhost:5432/db")
	require.NoError(t, os.Unsetenv("RUN_ADDRESS"))
	require.NoError(t, os.Unsetenv("JWT_SECRET"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_POLL_INTERVAL"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_BATCH_SIZE"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_LOCK_TTL"))
	require.NoError(t, os.Unsetenv("LOG_FORMAT"))
	require.NoError(t, os.Unsetenv("LOG_LEVEL"))
	require.NoError(t, os.Unsetenv("LOG_ADD_SOURCE"))

	// Act
	cfg, err := GetConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", cfg.RunAddr)
	assert.Equal(t, "127.0.0.1:8081", cfg.AccrualAddr)
	assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.DatabaseURI)
	assert.Equal(t, "secret", cfg.JWTSecret)
	assert.Equal(t, 2*time.Second, cfg.AccrualPollInterval)
	assert.Equal(t, 10, cfg.AccrualBatchSize)
	assert.Equal(t, 5*time.Minute, cfg.AccrualLockTTL)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, false, cfg.Logging.AddSource)
}

// TestGetConfig_Flags проверяет парсинг параметров командной строки сервиса.
func TestGetConfig_Flags(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{
		"cmd",
		"-a=localhost:7777",
		"-r=localhost:8888",
		"-d=dburi",
		"-j=jwt",
		"-p=3s",
		"-b=10",
		"-l=30s",
		"--log-format=text",
		"--log-level=debug",
		"--log-add-source=true",
	}
	require.NoError(t, os.Unsetenv("RUN_ADDRESS"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS"))
	require.NoError(t, os.Unsetenv("DATABASE_URI"))
	require.NoError(t, os.Unsetenv("JWT_SECRET"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_POLL_INTERVAL"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_BATCH_SIZE"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_LOCK_TTL"))
	require.NoError(t, os.Unsetenv("LOG_FORMAT"))
	require.NoError(t, os.Unsetenv("LOG_LEVEL"))
	require.NoError(t, os.Unsetenv("LOG_ADD_SOURCE"))

	// Act
	cfg, err := GetConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:7777", cfg.RunAddr)
	assert.Equal(t, "localhost:8888", cfg.AccrualAddr)
	assert.Equal(t, "dburi", cfg.DatabaseURI)
	assert.Equal(t, "jwt", cfg.JWTSecret)
	assert.Equal(t, 3*time.Second, cfg.AccrualPollInterval)
	assert.Equal(t, 10, cfg.AccrualBatchSize)
	assert.Equal(t, 30*time.Second, cfg.AccrualLockTTL)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, true, cfg.Logging.AddSource)
}

// TestGetConfig_LongNameFlags проверяет парсинг параметров командной строки сервиса, заданных длинными именами.
func TestGetConfig_LongNameFlags(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{
		"cmd",
		"--run-address=localhost:7777",
		"--accrual-address=localhost:8888",
		"--database-uri=dburi",
		"--jwt-secret=jwt",
		"--accrual-poll-interval=3s",
		"--accrual-batch-size=10",
		"--accrual-lock-ttl=30s",
		"--log-format=text",
		"--log-level=debug",
		"--log-add-source=true",
	}
	require.NoError(t, os.Unsetenv("RUN_ADDRESS"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS"))
	require.NoError(t, os.Unsetenv("DATABASE_URI"))
	require.NoError(t, os.Unsetenv("JWT_SECRET"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_POLL_INTERVAL"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_BATCH_SIZE"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_LOCK_TTL"))
	require.NoError(t, os.Unsetenv("LOG_FORMAT"))
	require.NoError(t, os.Unsetenv("LOG_LEVEL"))
	require.NoError(t, os.Unsetenv("LOG_ADD_SOURCE"))

	// Act
	cfg, err := GetConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:7777", cfg.RunAddr)
	assert.Equal(t, "localhost:8888", cfg.AccrualAddr)
	assert.Equal(t, "dburi", cfg.DatabaseURI)
	assert.Equal(t, "jwt", cfg.JWTSecret)
	assert.Equal(t, 3*time.Second, cfg.AccrualPollInterval)
	assert.Equal(t, 10, cfg.AccrualBatchSize)
	assert.Equal(t, 30*time.Second, cfg.AccrualLockTTL)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, true, cfg.Logging.AddSource)
}

// TestGetConfig_Env проверяет парсинг переменных окружения сервиса.
func TestGetConfig_Env(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"cmd"}
	t.Setenv("RUN_ADDRESS", "127.0.0.1:9999")
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "127.0.0.1:8081")
	t.Setenv("DATABASE_URI", "postgres://user:pass@localhost:5432/db")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ACCRUAL_SYSTEM_POLL_INTERVAL", "1500ms")
	t.Setenv("ACCRUAL_SYSTEM_BATCH_SIZE", "77")
	t.Setenv("ACCRUAL_SYSTEM_LOCK_TTL", "1m")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_ADD_SOURCE", "true")

	// Act
	cfg, err := GetConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9999", cfg.RunAddr)
	assert.Equal(t, "127.0.0.1:8081", cfg.AccrualAddr)
	assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.DatabaseURI)
	assert.Equal(t, "secret", cfg.JWTSecret)
	assert.Equal(t, 1500*time.Millisecond, cfg.AccrualPollInterval)
	assert.Equal(t, 77, cfg.AccrualBatchSize)
	assert.Equal(t, 1*time.Minute, cfg.AccrualLockTTL)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, true, cfg.Logging.AddSource)
}

// TestGetConfig_EnvPrecedence проверяет приоритет переменных окружения над параметрами командной строки сервиса.
func TestGetConfig_EnvPrecedence(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{
		"cmd",
		"-a=localhost:7777",
		"-r=localhost:8888",
		"-d=dburi",
		"-j=jwt",
		"-p=3s",
		"-b=10",
		"-l=3m",
		"--log-format=json",
		"--log-level=warn",
		"--log-level=true",
	}
	t.Setenv("RUN_ADDRESS", "127.0.0.1:9999")
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "127.0.0.1:8081")
	t.Setenv("DATABASE_URI", "postgres://user:pass@localhost:5432/db")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ACCRUAL_SYSTEM_POLL_INTERVAL", "1500ms")
	t.Setenv("ACCRUAL_SYSTEM_BATCH_SIZE", "77")
	t.Setenv("ACCRUAL_SYSTEM_LOCK_TTL", "1m")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_ADD_SOURCING", "false")

	// Act
	cfg, err := GetConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9999", cfg.RunAddr)
	assert.Equal(t, "127.0.0.1:8081", cfg.AccrualAddr)
	assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.DatabaseURI)
	assert.Equal(t, "secret", cfg.JWTSecret)
	assert.Equal(t, 1500*time.Millisecond, cfg.AccrualPollInterval)
	assert.Equal(t, 77, cfg.AccrualBatchSize)
	assert.Equal(t, 1*time.Minute, cfg.AccrualLockTTL)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, false, cfg.Logging.AddSource)
}

// TestGetConfig_AccrualAddrRequired проверяет обязательность адреса сервиса расчета начислений баллов лояльности.
func TestGetConfig_AccrualAddrRequired(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"cmd"}
	t.Setenv("DATABASE_URI", "postgres://user:pass@localhost:5432/db")
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS"))
	require.NoError(t, os.Unsetenv("RUN_ADDRESS"))
	require.NoError(t, os.Unsetenv("JWT_SECRET"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_POLL_INTERVAL"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_BATCH_SIZE"))
	require.NoError(t, os.Unsetenv("LOG_FORMAT"))
	require.NoError(t, os.Unsetenv("LOG_LEVEL"))
	require.NoError(t, os.Unsetenv("LOG_ADD_SOURCE"))

	// Act
	_, err := GetConfig()

	// Assert
	require.Error(t, err)
}

// TestGetConfig_DatabaseURIRequired проверяет обязательность строки подключения к БД.
func TestGetConfig_DatabaseURIRequired(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"cmd"}
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "127.0.0.1:8081")
	require.NoError(t, os.Unsetenv("DATABASE_URI"))
	require.NoError(t, os.Unsetenv("RUN_ADDRESS"))
	require.NoError(t, os.Unsetenv("JWT_SECRET"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_POLL_INTERVAL"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_BATCH_SIZE"))
	require.NoError(t, os.Unsetenv("LOG_FORMAT"))
	require.NoError(t, os.Unsetenv("LOG_LEVEL"))
	require.NoError(t, os.Unsetenv("LOG_ADD_SOURCE"))

	// Act
	_, err := GetConfig()

	// Assert
	require.Error(t, err)
}

// TestGetConfig_RunAddrInvalid проверяет ошибку при некорректном адресе запуска HTTP-сервера.
func TestGetConfig_RunAddrInvalid(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"cmd"}
	t.Setenv("RUN_ADDRESS", "localhost:8080/path")
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "127.0.0.1:8081")
	t.Setenv("DATABASE_URI", "postgres://user:pass@localhost:5432/db")
	require.NoError(t, os.Unsetenv("JWT_SECRET"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_POLL_INTERVAL"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_BATCH_SIZE"))
	require.NoError(t, os.Unsetenv("LOG_FORMAT"))
	require.NoError(t, os.Unsetenv("LOG_LEVEL"))
	require.NoError(t, os.Unsetenv("LOG_ADD_SOURCE"))

	// Act
	_, err := GetConfig()

	// Assert
	require.Error(t, err)
}

// TestGetConfig_AccrualAddrInvalid проверяет ошибку при некорректном адресе сервиса расчета начислений баллов
// лояльности.
func TestGetConfig_AccrualAddrInvalid(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"cmd"}
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "127.0.0.1:8081/path")
	t.Setenv("DATABASE_URI", "postgres://user:pass@localhost:5432/db")
	require.NoError(t, os.Unsetenv("RUN_ADDRESS"))
	require.NoError(t, os.Unsetenv("JWT_SECRET"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_POLL_INTERVAL"))
	require.NoError(t, os.Unsetenv("ACCRUAL_SYSTEM_BATCH_SIZE"))
	require.NoError(t, os.Unsetenv("LOG_FORMAT"))
	require.NoError(t, os.Unsetenv("LOG_LEVEL"))
	require.NoError(t, os.Unsetenv("LOG_ADD_SOURCE"))

	// Act
	_, err := GetConfig()

	// Assert
	require.Error(t, err)
}

// Test_validateHttpAddr_OK проверяет, что адреса в корректном формате проходят валидацию.
func Test_validateHttpAddr_OK(t *testing.T) {
	// Arrange
	addrs := []string{
		"localhost",
		"localhost:8080",
		"127.0.0.1",
		"127.0.0.1:9091",
		"http://localhost",
		"http://localhost:8080",
		"https://127.0.0.1",
		"https://127.0.0.1:9091",
	}

	for _, addr := range addrs {
		t.Run(addr, func(t *testing.T) {
			// Act
			err := validateHTTPAddr(addr)

			// Assert
			require.NoError(t, err)
		})
	}
}

// Test_validateHttpAddr_ErrorOnInvalidFormat проверяет ошибку при некорректном формате адреса.
func Test_validateHttpAddr_ErrorOnInvalidFormat(t *testing.T) {
	// Arrange
	addr := "a:b:c"

	// Act
	err := validateHTTPAddr(addr)

	// Assert
	require.Error(t, err)
}

// Test_validateHttpAddr_ErrorOnPathOrQuery проверяет ошибку при наличии path/query или фрагмента в адресе.
func Test_validateHttpAddr_ErrorOnPathOrQuery(t *testing.T) {
	// Arrange
	addrs := []string{
		"localhost:8080/path",
		"http://localhost:8080/path",
		"localhost:8080?x=1",
		"https://localhost:8080?x=1",
		"http://localhost:8080/#frag",
	}

	for _, addr := range addrs {
		t.Run(addr, func(t *testing.T) {
			// Act
			err := validateHTTPAddr(addr)

			// Assert
			require.Error(t, err)
		})
	}
}
