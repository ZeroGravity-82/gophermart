.PHONY: test test-integration accrual-integration db-test-up db-test-down fmt lint help

# DSN по умолчанию для локальных интеграционных тестов (compose.test.yaml).
TEST_DATABASE_URI ?= postgres://gophermart:userpassword@localhost:15432/gophermart_test?sslmode=disable

help: ## Показать доступные цели
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "%-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Отформатировать Go-файлы
	gofmt -w ./cmd ./internal

test: ## Запустить unit-тесты
	go test ./...

lint: ## Запустить базовые статические проверки (go vet)
	go vet ./...

# --- Интеграционная БД (Docker Compose) ---

db-test-up: ## Запустить изолированный PostgreSQL для интеграционных тестов
	docker compose --project-name gophermart-itest -f compose.test.yaml up -d

db-test-down: ## Остановить и удалить контейнер с PostgreSQL для интеграционных тестов
	docker compose --project-name gophermart-itest -f compose.test.yaml down

# --- Интеграционные тесты ---

test-integration: db-test-up ## Запустить интеграционные тесты PostgreSQL
	TEST_DATABASE_URI='$(TEST_DATABASE_URI)' go test -tags=integration ./internal/repository/postgresql

accrual-integration: ## Запустить интеграционные тесты accrual-клиента
	go test -tags=integration ./internal/accrual
