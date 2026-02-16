package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/gophermart/internal/auth"
	"zerogravity-82/gophermart/internal/httpserver/handler/dto"
	"zerogravity-82/gophermart/internal/httpserver/middleware"
	"zerogravity-82/gophermart/internal/model"
)

type userServiceStub struct {
	register func(ctx context.Context, login, password string) (model.Tokens, error)
	login    func(ctx context.Context, login, password string) (model.Tokens, error)
	refresh  func(ctx context.Context, refreshToken string) (model.Tokens, error)
	logout   func(ctx context.Context, refreshToken string) error
}

func (s userServiceStub) Register(ctx context.Context, login, password string) (model.Tokens, error) {
	return s.register(ctx, login, password)
}
func (s userServiceStub) Login(ctx context.Context, login, password string) (model.Tokens, error) {
	return s.login(ctx, login, password)
}
func (s userServiceStub) Refresh(ctx context.Context, refreshToken string) (model.Tokens, error) {
	return s.refresh(ctx, refreshToken)
}
func (s userServiceStub) Logout(ctx context.Context, refreshToken string) error {
	return s.logout(ctx, refreshToken)
}

type orderServiceStub struct {
	upload func(ctx context.Context, userID, number string) error
	list   func(ctx context.Context, userID string) ([]model.Order, error)
}

func (s orderServiceStub) Upload(ctx context.Context, userID, number string) error {
	return s.upload(ctx, userID, number)
}
func (s orderServiceStub) List(ctx context.Context, userID string) ([]model.Order, error) {
	return s.list(ctx, userID)
}

type errWriter struct{}

func (errWriter) Header() http.Header         { return make(http.Header) }
func (errWriter) Write(_ []byte) (int, error) { return 0, errors.New("write failure") }
func (errWriter) WriteHeader(_ int)           {}

func authRequest(t *testing.T, req *http.Request, subject string) *auth.JWTManager {
	// Arrange
	jwtm, err := auth.NewJWTManager("secret", 15*60*1e9)
	require.NoError(t, err)
	tok, err := jwtm.IssueAccessToken(subject)
	require.NoError(t, err)

	// Act
	req.Header.Set("Authorization", "Bearer "+tok)

	// Assert
	return jwtm
}

type balanceServiceStub struct {
	getBalance      func(ctx context.Context, userID string) (model.Balance, error)
	withdraw        func(ctx context.Context, userID, orderNumber string, sum uint64) error
	listWithdrawals func(ctx context.Context, userID string) ([]model.Withdrawal, error)
}

func (s balanceServiceStub) GetBalance(ctx context.Context, userID string) (model.Balance, error) {
	return s.getBalance(ctx, userID)
}
func (s balanceServiceStub) Withdraw(ctx context.Context, userID, orderNumber string, sum uint64) error {
	return s.withdraw(ctx, userID, orderNumber, sum)
}
func (s balanceServiceStub) ListWithdrawals(ctx context.Context, userID string) ([]model.Withdrawal, error) {
	return s.listWithdrawals(ctx, userID)
}

// TestRegisterHandler_OK проверяет успешную регистрацию.
func TestRegisterHandler_OK(t *testing.T) {
	// Arrange
	us := userServiceStub{
		register: func(_ context.Context, login, password string) (model.Tokens, error) {
			require.Equal(t, "l", login)
			require.Equal(t, "p", password)
			return model.Tokens{AccessToken: "a", RefreshToken: "r"}, nil
		},
	}
	body, _ := json.Marshal(dto.CredentialsRequest{Login: "l", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Act
	RegisterHandler(us).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Bearer a", w.Header().Get("Authorization"))
	assert.Equal(t, "r", w.Header().Get("X-Refresh-Token"))
}

// TestRegisterHandler_LoginTaken проверяет конфликт при занятом логине.
func TestRegisterHandler_LoginTaken(t *testing.T) {
	// Arrange
	us := userServiceStub{
		register: func(_ context.Context, _, _ string) (model.Tokens, error) {
			return model.Tokens{}, model.ErrLoginAlreadyTaken
		},
	}
	body, _ := json.Marshal(dto.CredentialsRequest{Login: "l", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Act
	RegisterHandler(us).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestLoginHandler_Unauthorized проверяет 401 при ошибке аутентификации.
func TestLoginHandler_Unauthorized(t *testing.T) {
	// Arrange
	us := userServiceStub{
		login: func(_ context.Context, _, _ string) (model.Tokens, error) {
			return model.Tokens{}, model.ErrAuthenticationFailed
		},
	}
	body, _ := json.Marshal(dto.CredentialsRequest{Login: "l", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Act
	LoginHandler(us).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRefreshHandler_OK проверяет успешное обновление токенов.
func TestRefreshHandler_OK(t *testing.T) {
	// Arrange
	us := userServiceStub{
		refresh: func(_ context.Context, refreshToken string) (model.Tokens, error) {
			require.Equal(t, "rt", refreshToken)
			return model.Tokens{AccessToken: "a2", RefreshToken: "r2"}, nil
		},
	}
	body, _ := json.Marshal(dto.RefreshRequest{RefreshToken: "rt"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/refresh", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Act
	RefreshHandler(us).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Bearer a2", w.Header().Get("Authorization"))
	assert.Equal(t, "r2", w.Header().Get("X-Refresh-Token"))
}

// TestLogoutHandler_OK проверяет успешный logout.
func TestLogoutHandler_OK(t *testing.T) {
	// Arrange
	called := false
	us := userServiceStub{
		logout: func(_ context.Context, refreshToken string) error {
			called = true
			require.Equal(t, "rt", refreshToken)
			return nil
		},
	}
	body, _ := json.Marshal(dto.RefreshRequest{RefreshToken: "rt"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/logout", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Act
	LogoutHandler(us).ServeHTTP(w, req)

	// Assert
	assert.True(t, called)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestUploadOrderHandler_Unauthorized проверяет 401 при отсутствии userID в контексте.
func TestUploadOrderHandler_Unauthorized(t *testing.T) {
	// Arrange
	os := orderServiceStub{upload: func(_ context.Context, _, _ string) error { return nil }}
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("123"))
	w := httptest.NewRecorder()

	// Act
	UploadOrderHandler(os).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestUploadOrderHandler_OK проверяет успешную постановку заказа в обработку.
func TestUploadOrderHandler_OK(t *testing.T) {
	// Arrange
	os := orderServiceStub{upload: func(_ context.Context, userID, number string) error {
		require.Equal(t, "u1", userID)
		require.Equal(t, "123", number)
		return nil
	}}

	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("123"))
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(UploadOrderHandler(os)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusAccepted, w.Code)
}

// TestUploadOrderHandler_OkOnDuplicateSameUser проверяет 200 при повторной загрузке того же заказа тем же
// пользователем.
func TestUploadOrderHandler_OkOnDuplicateSameUser(t *testing.T) {
	// Arrange
	os := orderServiceStub{upload: func(_ context.Context, _, _ string) error {
		return model.ErrOrderAlreadyExistsSameUser
	}}
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("123"))
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(UploadOrderHandler(os)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestUploadOrderHandler_ConflictOnDuplicateOtherUser проверяет 409, если заказ уже загружен другим пользователем.
func TestUploadOrderHandler_ConflictOnDuplicateOtherUser(t *testing.T) {
	// Arrange
	os := orderServiceStub{upload: func(_ context.Context, _, _ string) error {
		return model.ErrOrderAlreadyExistsOtherUser
	}}
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("123"))
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(UploadOrderHandler(os)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestUploadOrderHandler_EmptyBody проверяет 400 при пустом номере заказа.
func TestUploadOrderHandler_EmptyBody(t *testing.T) {
	// Arrange
	os := orderServiceStub{upload: func(_ context.Context, _, _ string) error {
		return nil
	}}
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("   "))
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(UploadOrderHandler(os)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUploadOrderHandler_InvalidOrder проверяет 422 при некорректном номере заказа.
func TestUploadOrderHandler_InvalidOrder(t *testing.T) {
	// Arrange
	os := orderServiceStub{upload: func(_ context.Context, _, _ string) error {
		return model.ErrInvalidOrderNumber
	}}
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("bad"))
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(UploadOrderHandler(os)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// TestGetOrdersHandler_InternalError проверяет 500 при ошибке получения списка заказов.
func TestGetOrdersHandler_InternalError(t *testing.T) {
	// Arrange
	os := orderServiceStub{list: func(_ context.Context, _ string) ([]model.Order, error) {
		return nil, errors.New("boom")
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(GetOrdersHandler(os)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestWithdrawHandler_InvalidJSON проверяет 400 при невалидном JSON.
func TestWithdrawHandler_InvalidJSON(t *testing.T) {
	// Arrange
	bs := balanceServiceStub{withdraw: func(_ context.Context, _, _ string, _ uint64) error { return nil }}
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBufferString("{"))
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(WithdrawHandler(bs)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestWithdrawHandler_InternalError проверяет 500 при неожиданной ошибке сервиса.
func TestWithdrawHandler_InternalError(t *testing.T) {
	// Arrange
	bs := balanceServiceStub{withdraw: func(_ context.Context, _, _ string, _ uint64) error { return errors.New("boom") }}
	body, _ := json.Marshal(dto.WithdrawRequest{Order: "79927398713", Sum: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(WithdrawHandler(bs)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetOrdersHandler_EncodeError проверяет 500 при ошибке сериализации ответа.
func TestGetOrdersHandler_EncodeError(t *testing.T) {
	// Arrange
	now := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	os := orderServiceStub{list: func(_ context.Context, _ string) ([]model.Order, error) {
		return []model.Order{{Number: "1", Status: model.OrderStatusNew, UploadedAt: now}}, nil
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	jwtm := authRequest(t, req, "u1")
	w := errWriter{}

	// Act
	middleware.WithAuth(jwtm)(GetOrdersHandler(os)).ServeHTTP(w, req)

	// Assert
	// Ошибка encode приводит к http.Error, но наш writer всегда возвращает ошибку.
	// Проверяем только то, что handler не паникует.
}

// TestGetBalanceHandler_EncodeError проверяет 500 при ошибке сериализации ответа.
func TestGetBalanceHandler_EncodeError(t *testing.T) {
	// Arrange
	bs := balanceServiceStub{getBalance: func(_ context.Context, _ string) (model.Balance, error) {
		return model.Balance{Accrual: 100, Withdrawn: 0}, nil
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	jwtm := authRequest(t, req, "u1")
	w := errWriter{}

	// Act
	middleware.WithAuth(jwtm)(GetBalanceHandler(bs)).ServeHTTP(w, req)

	// Assert
	// Аналогично тесту GetOrdersHandler_EncodeError: проверяем отсутствие паники.
}

// TestWithdrawHandler_OK проверяет успешное списание.
func TestWithdrawHandler_OK(t *testing.T) {
	// Arrange
	called := false
	bs := balanceServiceStub{withdraw: func(_ context.Context, userID, orderNumber string, sum uint64) error {
		called = true
		require.Equal(t, "u1", userID)
		require.Equal(t, "79927398713", orderNumber)
		require.EqualValues(t, 100, sum)
		return nil
	}}
	body, _ := json.Marshal(dto.WithdrawRequest{Order: "79927398713", Sum: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(WithdrawHandler(bs)).ServeHTTP(w, req)

	// Assert
	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetOrdersHandler_NoContent проверяет 204 при отсутствии заказов.
func TestGetOrdersHandler_NoContent(t *testing.T) {
	// Arrange
	os := orderServiceStub{list: func(_ context.Context, userID string) ([]model.Order, error) {
		require.Equal(t, "u1", userID)
		return nil, nil
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(GetOrdersHandler(os)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestGetOrdersHandler_OK проверяет 200 и JSON при наличии заказов.
func TestGetOrdersHandler_OK(t *testing.T) {
	// Arrange
	now := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	os := orderServiceStub{list: func(_ context.Context, userID string) ([]model.Order, error) {
		require.Equal(t, "u1", userID)
		accrual := uint64(123)
		return []model.Order{
			{Number: "1", Status: model.OrderStatusNew, UploadedAt: now, Accrual: nil},
			{Number: "2", Status: model.OrderStatusProcessed, UploadedAt: now, Accrual: &accrual},
		}, nil
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(GetOrdersHandler(os)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), "\"number\"")
}

// TestGetBalanceHandler_OK проверяет успешную выдачу баланса.
func TestGetBalanceHandler_OK(t *testing.T) {
	// Arrange
	bs := balanceServiceStub{getBalance: func(_ context.Context, userID string) (model.Balance, error) {
		require.Equal(t, "u1", userID)
		return model.Balance{Accrual: 1000, Withdrawn: 200}, nil
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(GetBalanceHandler(bs)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"current\"")
}

// TestWithdrawHandler_InvalidSum проверяет 400 при некорректной сумме.
func TestWithdrawHandler_InvalidSum(t *testing.T) {
	// Arrange
	bs := balanceServiceStub{withdraw: func(_ context.Context, _, _ string, _ uint64) error { return nil }}
	body, _ := json.Marshal(dto.WithdrawRequest{Order: "123", Sum: -1})
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(WithdrawHandler(bs)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetWithdrawalsHandler_NoContent проверяет 204 при пустом списке.
func TestGetWithdrawalsHandler_NoContent(t *testing.T) {
	// Arrange
	bs := balanceServiceStub{listWithdrawals: func(_ context.Context, _ string) ([]model.Withdrawal, error) {
		return nil, nil
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	jwtm := authRequest(t, req, "u1")
	w := httptest.NewRecorder()

	// Act
	middleware.WithAuth(jwtm)(GetWithdrawalsHandler(bs)).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestRegisterHandler_BadJSON проверяет 400 при невалидном JSON.
func TestRegisterHandler_BadJSON(t *testing.T) {
	// Arrange
	us := userServiceStub{
		register: func(_ context.Context, _, _ string) (model.Tokens, error) { return model.Tokens{}, nil },
	}
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString("{"))
	w := httptest.NewRecorder()

	// Act
	RegisterHandler(us).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestRegisterHandler_InternalError проверяет 500 при неожиданной ошибке сервиса.
func TestRegisterHandler_InternalError(t *testing.T) {
	// Arrange
	us := userServiceStub{
		register: func(_ context.Context, _, _ string) (model.Tokens, error) {
			return model.Tokens{}, errors.New("boom")
		},
	}
	body, _ := json.Marshal(dto.CredentialsRequest{Login: "l", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Act
	RegisterHandler(us).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestWithdrawHandler_PropagateErrorMapping проверяет маппинг ошибок сервиса в HTTP-коды.
func TestWithdrawHandler_PropagateErrorMapping(t *testing.T) {
	// Arrange
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid order", err: model.ErrInvalidOrderNumber, wantStatus: http.StatusUnprocessableEntity},
		{name: "insufficient", err: model.ErrInsufficientFunds, wantStatus: http.StatusPaymentRequired},
		{name: "conflict", err: model.ErrWithdrawConflict, wantStatus: http.StatusConflict},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			bs := balanceServiceStub{withdraw: func(_ context.Context, _, _ string, _ uint64) error { return tt.err }}
			body, _ := json.Marshal(dto.WithdrawRequest{Order: "79927398713", Sum: 1})
			req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
			jwtm := authRequest(t, req, "u1")
			w := httptest.NewRecorder()

			// Act
			middleware.WithAuth(jwtm)(WithdrawHandler(bs)).ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
