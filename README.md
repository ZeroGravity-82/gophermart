# Gophermart

Накопительная система лояльности «Гофермарт».

## Возможности

- Регистрация и вход пользователей
- Аутентификация по JWT
- Refresh-token с ротацией и logout
- Работа с заказами, балансом и списанием баллов (ручки требуют авторизации)

## Стек

- Go
- PostgreSQL

## Конфигурация

Сервис читает параметры из переменных окружения и/или флагов командной строки.
Приоритет: переменные окружения имеют приоритет над флагами.

### Переменные окружения

- `DATABASE_URI` — строка подключения к PostgreSQL (обязательная)
- `ACCRUAL_SYSTEM_ADDRESS` — адрес accrual-сервиса (обязательная, например `http://localhost:8081`)
- `RUN_ADDRESS` — адрес HTTP-сервера (опционально, default: `localhost:8080`)
- `ACCRUAL_SYSTEM_POLL_INTERVAL` — интервал опроса accrual-сервиса (опционально, default: `2s`)
- `ACCRUAL_SYSTEM_BATCH_SIZE` — размер пачки заказов для опроса accrual-сервиса (опционально, default: `10`)
- `ACCRUAL_SYSTEM_LOCK_TTL` — TTL блокировки заказов воркером опроса accrual-сервиса (опционально, default: `5m`)
- `JWT_SECRET` — секрет для подписи JWT (опционально, default: `secret`)

### Флаги

- `-d` — строка подключения к PostgreSQL (аналог `DATABASE_URI`)
- `-r` — адрес accrual-сервиса (аналог `ACCRUAL_SYSTEM_ADDRESS`)
- `-a` — адрес HTTP-сервера (аналог `RUN_ADDRESS`)
- `-p` — интервал опроса accrual-сервиса (аналог `ACCRUAL_SYSTEM_POLL_INTERVAL`)
- `-b` — размер пачки заказов для опроса accrual-сервиса (аналог `ACCRUAL_SYSTEM_BATCH_SIZE`)
- `-l` — TTL блокировки заказов воркером опроса accrual-сервиса (аналог `ACCRUAL_SYSTEM_LOCK_TTL`)
- `-j` — секрет для подписи JWT (аналог `JWT_SECRET`)

## HTTP API

Актуальная спецификация: `api/openapi.yaml`.

### Аутентификация

`POST /api/user/register`, `POST /api/user/login`, `POST /api/user/refresh` возвращают токены через заголовки ответа:

- `Authorization: Bearer <access_token>`
- `X-Refresh-Token: <refresh_token>`

`POST /api/user/logout` принимает refresh-токен и отзывает его (204).

Тела запросов:

- `POST /api/user/register` — `{"login":"...","password":"..."}`
- `POST /api/user/login` — `{"login":"...","password":"..."}`
- `POST /api/user/refresh` — `{"refresh_token":"..."}`
- `POST /api/user/logout` — `{"refresh_token":"..."}`

### Защищённые ручки

Следующие ручки требуют `Authorization: Bearer <access_token>`:

- `POST /api/user/orders` — загрузка номера заказа (тело `text/plain`)
- `GET /api/user/orders` — список загруженных номеров заказов
- `GET /api/user/balance` — текущий баланс пользователя
- `POST /api/user/balance/withdraw` — списание средств (тело `application/json`)
- `GET /api/user/withdrawals` — информация о выводе средств

Ошибки возвращаются как `text/plain`.

## Локальный запуск

Требования:

- PostgreSQL
- Go (поддерживаемая версией `go.mod`)

Миграции:

- Миграции лежат в `migrations/` и применяются автоматически при старте сервиса.

### Запуск через env

```bash
export DATABASE_URI='postgres://user:pass@localhost:5432/gophermart?sslmode=disable'
export ACCRUAL_SYSTEM_ADDRESS='http://localhost:8081'
export RUN_ADDRESS='localhost:8080'

go run ./cmd/gophermart
```

### Запуск через флаги

```bash
go run ./cmd/gophermart -d 'postgres://user:pass@localhost:5432/gophermart?sslmode=disable' \
  -r 'http://localhost:8081' \
  -a 'localhost:8080'
```


## Интеграционные тесты (PostgreSQL)

Интеграционные тесты для слоя PostgreSQL-репозиториев лежат в `internal/repository/postgresql` и защищены build-тегом `integration`.

### Через Makefile (рекомендуется)

```bash
make test-integration
```

По умолчанию используется DSN:

- `postgres://gophermart:userpassword@localhost:15432/gophermart_test?sslmode=disable`

Переопределить можно так:

```bash
make test-integration TEST_DATABASE_URI='postgres://...'
```

### Вручную

1) Запустить PostgreSQL для тестов (изолированная БД/порт):

```bash
docker compose -f compose.test.yaml up -d
```

2) Запустить интеграционные тесты (DSN передается через переменную окружения):

```bash
TEST_DATABASE_URI='postgres://gophermart:userpassword@localhost:15432/gophermart_test?sslmode=disable' \
  go test -tags=integration ./internal/repository/postgresql
```

Примечания:
- Тесты автоматически применяют миграции из `migrations/` и затем делают `TRUNCATE` таблиц для чистого состояния.
- Если `TEST_DATABASE_URI` не задана, интеграционные тесты будут пропущены.

## Интеграционные тесты (accrual-сервис)

Интеграционные тесты HTTP-клиента accrual-сервиса лежат в `internal/accrual` и также защищены build-тегом `integration`.

### Через Makefile (рекомендуется)

```bash
make accrual-integration
```

### Вручную

```bash
go test -tags=integration ./internal/accrual
```

Примечания:
- Часть тестов поднимает реальный бинарник accrual-сервиса из `cmd/accrual` и требует доступной PostgreSQL (DSN берется из `TEST_DATABASE_URI`).
