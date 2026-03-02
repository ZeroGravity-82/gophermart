package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/auth"
	"zerogravity-82/gophermart/internal/httpserver/handler/dto"
	"zerogravity-82/gophermart/internal/repository/postgresql"
	"zerogravity-82/gophermart/internal/service"
)

// TestHTTPServer_Routes_RequireAuth проверяет, что защищенные ручки требуют JWT.
func TestHTTPServer_Routes_RequireAuth(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	// Для маршрутов нам не важна внутренняя логика сервисов, но нужны реальные типы, чтобы собрать HTTPServer.
	// Создаем сервисы на заглушках репозиториев.
	orderRepo, _ := postgresql.NewOrderRepo(sqlx.NewDb(nil, "pgx"), 1*time.Minute)
	orderSvc := service.NewOrderService(orderRepo)

	balanceRepo := postgresql.NewBalanceRepo(sqlx.NewDb(nil, "pgx"))
	balanceSvc := service.NewBalanceService(balanceRepo)

	userRepo := postgresql.NewUserRepo(sqlx.NewDb(nil, "pgx"))
	rtRepo := postgresql.NewRefreshTokenRepo(sqlx.NewDb(nil, "pgx"))
	userSvc := service.NewUserService(userRepo, rtRepo, jwtm)

	srv := New(":0", jwtm, userSvc, orderSvc, balanceSvc)
	r := srv.buildRouter()

	protected := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{name: "orders post", method: http.MethodPost, path: "/api/user/orders", body: []byte("123")},
		{name: "orders get", method: http.MethodGet, path: "/api/user/orders"},
		{name: "balance get", method: http.MethodGet, path: "/api/user/balance"},
		{
			name:   "withdraw post",
			method: http.MethodPost,
			path:   "/api/user/balance/withdraw",
			body:   []byte(`{"order":"1","sum":1}`),
		},
		{name: "withdrawals get", method: http.MethodGet, path: "/api/user/withdrawals"},
	}

	for _, tt := range protected {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(tt.body))
			w := httptest.NewRecorder()

			// Act
			r.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// TestHTTPServer_Register_AllowContentType проверяет, что /register принимает только application/json.
func TestHTTPServer_Register_AllowContentType(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	orderRepo, _ := postgresql.NewOrderRepo(sqlx.NewDb(nil, "pgx"), 1*time.Minute)
	orderSvc := service.NewOrderService(orderRepo)
	balanceRepo := postgresql.NewBalanceRepo(sqlx.NewDb(nil, "pgx"))
	balanceSvc := service.NewBalanceService(balanceRepo)
	userRepo := postgresql.NewUserRepo(sqlx.NewDb(nil, "pgx"))
	rtRepo := postgresql.NewRefreshTokenRepo(sqlx.NewDb(nil, "pgx"))
	userSvc := service.NewUserService(userRepo, rtRepo, jwtm)

	srv := New(":0", jwtm, userSvc, orderSvc, balanceSvc)
	r := srv.buildRouter()

	payload, _ := json.Marshal(dto.CredentialsRequest{Login: "l", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	// Act
	r.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

// TestHTTPServer_Run_CancelImmediately проверяет, что Run корректно завершается при отмене контекста.
func TestHTTPServer_Run_CancelImmediately(t *testing.T) {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*time.Minute)
	require.NoError(t, err)

	orderRepo, _ := postgresql.NewOrderRepo(sqlx.NewDb(nil, "pgx"), 1*time.Minute)
	orderSvc := service.NewOrderService(orderRepo)
	balanceRepo := postgresql.NewBalanceRepo(sqlx.NewDb(nil, "pgx"))
	balanceSvc := service.NewBalanceService(balanceRepo)
	userRepo := postgresql.NewUserRepo(sqlx.NewDb(nil, "pgx"))
	rtRepo := postgresql.NewRefreshTokenRepo(sqlx.NewDb(nil, "pgx"))
	userSvc := service.NewUserService(userRepo, rtRepo, jwtm)

	srv := New("127.0.0.1:0", jwtm, userSvc, orderSvc, balanceSvc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	err = srv.Run(ctx)

	// Assert
	require.NoError(t, err)
}
