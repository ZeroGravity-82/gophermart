// Package config читает и валидирует конфигурацию сервиса (env и флаги).
package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	defaultRunAddr             = "localhost:8080"
	defaultAccrualPollInterval = 2 * time.Second
	defaultAccrualBatchSize    = 10
	defaultAccrualLockTTL      = 5 * time.Minute
	defaultJWTSecret           = "secret"
)

// Config содержит параметры конфигурации сервиса.
//
// Значения берутся из переменных окружения и/или флагов командной строки.
type Config struct {
	RunAddr             string
	AccrualAddr         string
	AccrualPollInterval time.Duration
	AccrualBatchSize    int
	AccrualLockTTL      time.Duration
	DatabaseURI         string
	JWTSecret           string
}

// GetConfig читает конфигурацию из переменных окружения/флагов командной строки и возвращает итоговый Config.
func GetConfig() (Config, error) {
	var runAddrFlag, accrualAddrFlag string
	flag.Func("a", runAddrUsage(), httpAddrFlagParser(&runAddrFlag))
	flag.Func("r", "адрес системы расчета начислений", httpAddrFlagParser(&accrualAddrFlag))
	databaseURIFlag := flag.String("d", "", "строка подключения к базе данных")
	jwtSecretFlag := flag.String("j", defaultJWTSecret, "секрет для подписи JWT")
	accrualPollIntervalFlag := flag.Duration(
		"p",
		defaultAccrualPollInterval,
		"интервал опроса сервиса расчета начислений баллов лояльности",
	)
	accrualBatchSizeFlag := flag.Int(
		"b",
		defaultAccrualBatchSize,
		"размер пачки заказов для опроса сервиса расчета начислений баллов лояльности",
	)
	accrualLockTTLFlag := flag.Duration(
		"l",
		defaultAccrualLockTTL,
		"длительность блокировки заказов воркером работы с сервисом расчета начислений баллов лояльности",
	)
	flag.Parse()

	cfg := Config{}
	runAddr, err := getRunAddr(runAddrFlag)
	if err != nil {
		return cfg, err
	}
	accrualAddr, err := getAccrualAddr(accrualAddrFlag)
	if err != nil {
		return cfg, err
	}
	accrualPollInterval, err := getAccrualPollInterval(*accrualPollIntervalFlag)
	if err != nil {
		return cfg, err
	}
	accrualBatchSize, err := getAccrualBatchSize(*accrualBatchSizeFlag)
	if err != nil {
		return cfg, err
	}
	accrualLockTTL, err := getAccrualLockTTL(*accrualLockTTLFlag)
	if err != nil {
		return cfg, err
	}
	databaseURI, err := getDatabaseURI(databaseURIFlag)
	if err != nil {
		return cfg, err
	}
	jwts := getJWTSecret(jwtSecretFlag)

	cfg.RunAddr = runAddr
	cfg.AccrualAddr = accrualAddr
	cfg.DatabaseURI = databaseURI
	cfg.JWTSecret = jwts
	cfg.AccrualPollInterval = accrualPollInterval
	cfg.AccrualBatchSize = accrualBatchSize
	cfg.AccrualLockTTL = accrualLockTTL
	return cfg, nil
}

func runAddrUsage() string {
	return fmt.Sprintf(`адрес и порт запуска сервиса (default "%s")`, defaultRunAddr)
}

func httpAddrFlagParser(httpAddr *string) func(string) error {
	return func(flagValue string) error {
		if err := validateHTTPAddr(flagValue); err != nil {
			return err
		}
		*httpAddr = flagValue
		return nil
	}
}

func validateHTTPAddr(v string) error {
	// Принимает обе следующие формы:
	//   host[:port]              (например, localhost:8080, 127.0.0.1)
	//   http(s)://host[:port]    (например, http://localhost:8080)
	// и отклоняет любые path/query-параметры и фрагменты.
	toParse := v
	if !hasScheme(v) {
		toParse = "http://" + v
	}

	parsedURL, err := url.Parse(toParse)
	if err != nil {
		return fmt.Errorf("invalid HTTP address: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("invalid HTTP address scheme")
	}
	if parsedURL.Host == "" {
		return errors.New("invalid HTTP address: empty host")
	}
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		return errors.New("HTTP address contains path parameters")
	}
	if parsedURL.RawQuery != "" {
		return errors.New("HTTP address contains query parameters")
	}
	if parsedURL.Fragment != "" {
		return errors.New("HTTP address contains fragment")
	}

	// Валидируем host[:port]. Порт необязателен.
	host := parsedURL.Hostname()
	if host == "" {
		return errors.New("invalid HTTP address: empty host")
	}
	if port := parsedURL.Port(); port != "" {
		// Проверяем порт.
		if _, err := url.Parse("http://" + host + ":" + port); err != nil {
			return fmt.Errorf("invalid HTTP address: %w", err)
		}
	}

	return nil
}

func hasScheme(v string) bool {
	u, err := url.Parse(v)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func getRunAddr(runAddrFlag string) (string, error) {
	runAddrEnv, ok := os.LookupEnv("RUN_ADDRESS")
	if !ok && runAddrFlag != "" {
		return runAddrFlag, nil
	}
	if !ok {
		return defaultRunAddr, nil
	}
	if err := validateHTTPAddr(runAddrEnv); err != nil {
		return "", fmt.Errorf("invalid service listen address: %w", err)
	}
	return runAddrEnv, nil
}

func getAccrualAddr(accrualAddrFlag string) (string, error) {
	accrualAddrEnv, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS")
	if !ok && accrualAddrFlag != "" {
		return accrualAddrFlag, nil
	}
	if !ok {
		return "", errors.New("accrual system address is not set")
	}
	if err := validateHTTPAddr(accrualAddrEnv); err != nil {
		return "", fmt.Errorf("invalid accrual system address: %w", err)
	}
	return accrualAddrEnv, nil
}

func getAccrualPollInterval(accrualPollIntervalFlag time.Duration) (time.Duration, error) {
	accrualPollIntervalEnvStr, ok := os.LookupEnv("ACCRUAL_SYSTEM_POLL_INTERVAL")
	if !ok {
		return accrualPollIntervalFlag, nil
	}
	accrualPollInterval, err := time.ParseDuration(accrualPollIntervalEnvStr)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to parse ACCRUAL_SYSTEM_POLL_INTERVAL environment variable value '%s' as duration: %w",
			accrualPollIntervalEnvStr,
			err,
		)
	}
	return accrualPollInterval, nil
}

func getAccrualBatchSize(accrualBatchSizeFlag int) (int, error) {
	accrualBatchSizeEnvStr, ok := os.LookupEnv("ACCRUAL_SYSTEM_BATCH_SIZE")
	if !ok {
		return accrualBatchSizeFlag, nil
	}
	accrualBatchSizeEnvInt, err := strconv.Atoi(accrualBatchSizeEnvStr)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to convert ACCRUAL_SYSTEM_BATCH_SIZE environment variable value '%s' to integer: %w",
			accrualBatchSizeEnvStr,
			err,
		)
	}
	return accrualBatchSizeEnvInt, nil
}

func getAccrualLockTTL(accrualLockTTLFlag time.Duration) (time.Duration, error) {
	accrualLockTTLEnvStr, ok := os.LookupEnv("ACCRUAL_SYSTEM_LOCK_TTL")
	if !ok {
		return accrualLockTTLFlag, nil
	}
	accrualLockTTL, err := time.ParseDuration(accrualLockTTLEnvStr)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to parse ACCRUAL_SYSTEM_LOCK_TTL environment variable value '%s' as duration: %w",
			accrualLockTTLEnvStr,
			err,
		)
	}
	return accrualLockTTL, nil
}

func getDatabaseURI(databaseURIFlag *string) (string, error) {
	databaseURIEnvStr, ok := os.LookupEnv("DATABASE_URI")
	if !ok && *databaseURIFlag != "" {
		return *databaseURIFlag, nil
	}
	if !ok {
		return "", errors.New("database URI is not set")
	}
	return databaseURIEnvStr, nil
}

func getJWTSecret(jwtSecretFlag *string) string {
	jwtSecretEnvStr, ok := os.LookupEnv("JWT_SECRET")
	if !ok && *jwtSecretFlag != "" {
		return *jwtSecretFlag
	}
	if !ok {
		return defaultJWTSecret
	}
	return jwtSecretEnvStr
}
