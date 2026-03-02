// Package handler содержит HTTP-обработчики.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"zerogravity-82/gophermart/internal/httpserver/handler/dto"
	"zerogravity-82/gophermart/internal/httpserver/middleware"
	"zerogravity-82/gophermart/internal/logging"
	"zerogravity-82/gophermart/internal/model"
)

const unauthorizedError = "unauthorized"

type userService interface {
	Register(ctx context.Context, login, password string) (model.Tokens, error)
	Login(ctx context.Context, login, password string) (model.Tokens, error)
	Refresh(ctx context.Context, refreshToken string) (model.Tokens, error)
	Logout(ctx context.Context, refreshToken string) error
}

type orderService interface {
	Upload(ctx context.Context, userID, number string) error
	List(ctx context.Context, userID string) ([]model.Order, error)
}

type balanceService interface {
	GetBalance(ctx context.Context, userID string) (model.Balance, error)
	Withdraw(ctx context.Context, userID, orderNumber string, sum uint64) error
	ListWithdrawals(ctx context.Context, userID string) ([]model.Withdrawal, error)
}

// RegisterHandler обрабатывает регистрацию пользователя.
//
// Ожидает JSON с логином и паролем, возвращает access/refresh токены в заголовках.
func RegisterHandler(us userService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dto.CredentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "cannot decode request body", http.StatusBadRequest)
			return
		}
		if err := validateCredentials(body.Login, body.Password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		tokens, err := us.Register(r.Context(), body.Login, body.Password)
		if err != nil {
			if errors.Is(err, model.ErrLoginAlreadyTaken) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			logging.FromContext(r.Context()).Error("register error", slog.Any("err", err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Authorization", "Bearer "+tokens.AccessToken)
		w.Header().Set("X-Refresh-Token", tokens.RefreshToken)
	}
}

func validateCredentials(login, password string) error {
	if login == "" {
		return errors.New("empty login")
	}
	if password == "" {
		return errors.New("empty password")
	}
	return nil
}

// LoginHandler обрабатывает аутентификацию пользователя.
//
// Ожидает JSON с логином и паролем, возвращает access/refresh токены в заголовках.
func LoginHandler(us userService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dto.CredentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "cannot decode request body", http.StatusBadRequest)
			return
		}
		if err := validateCredentials(body.Login, body.Password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		tokens, err := us.Login(r.Context(), body.Login, body.Password)
		if err != nil {
			if errors.Is(err, model.ErrAuthenticationFailed) {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			logging.FromContext(r.Context()).Error("login error", slog.Any("err", err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Authorization", "Bearer "+tokens.AccessToken)
		w.Header().Set("X-Refresh-Token", tokens.RefreshToken)
	}
}

// RefreshHandler выполняет обновление пары токенов по refresh-токену.
//
// Ожидает JSON с refresh-токеном, возвращает новую пару access/refresh токенов в заголовках.
func RefreshHandler(us userService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dto.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "cannot decode request body", http.StatusBadRequest)
			return
		}
		tokens, err := us.Refresh(r.Context(), body.RefreshToken)
		if err != nil {
			if errors.Is(err, model.ErrAuthenticationFailed) {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			logging.FromContext(r.Context()).Error("refresh error", slog.Any("err", err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Authorization", "Bearer "+tokens.AccessToken)
		w.Header().Set("X-Refresh-Token", tokens.RefreshToken)
	}
}

// LogoutHandler отзывает refresh-токен пользователя.
//
// Ожидает JSON с refresh-токеном; при успехе возвращает 204 No Content.
func LogoutHandler(us userService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dto.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "cannot decode request body", http.StatusBadRequest)
			return
		}
		if err := us.Logout(r.Context(), body.RefreshToken); err != nil {
			if errors.Is(err, model.ErrAuthenticationFailed) {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			logging.FromContext(r.Context()).Error("logout error", slog.Any("err", err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// UploadOrderHandler загружает номер заказа.
//
// Тело запроса должно быть text/plain с номером заказа.
func UploadOrderHandler(os orderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := middleware.UserIDFromContext(r.Context())
		if err != nil {
			http.Error(w, unauthorizedError, http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read request body", http.StatusBadRequest)
			return
		}

		number := strings.TrimSpace(string(body))
		if number == "" {
			http.Error(w, "empty order number", http.StatusBadRequest)
			return
		}
		err = os.Upload(r.Context(), userID, number)
		if err == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if errors.Is(err, model.ErrOrderAlreadyExistsSameUser) {
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, model.ErrOrderAlreadyExistsOtherUser) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if errors.Is(err, model.ErrInvalidOrderNumber) {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		logging.FromContext(r.Context()).Error("upload order error", slog.Any("err", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// GetOrdersHandler возвращает список загруженных номеров заказов.
//
// При отсутствии номеров заказов возвращает 204 No Content.
func GetOrdersHandler(os orderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := middleware.UserIDFromContext(r.Context())
		if err != nil {
			http.Error(w, unauthorizedError, http.StatusUnauthorized)
			return
		}

		orders, err := os.List(r.Context(), userID)
		if err != nil {
			logging.FromContext(r.Context()).Error("get orders error", slog.Any("err", err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if len(orders) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		resp := make([]dto.OrderResponse, 0, len(orders))
		for _, o := range orders {
			resp = append(resp, dto.ToOrderResponse(o))
		}

		w.Header().Set("Content-Type", "application/json")
		if err = json.NewEncoder(w).Encode(resp); err != nil {
			logging.FromContext(r.Context()).Error("get orders encode error", slog.Any("err", err))
			http.Error(w, "cannot encode response", http.StatusInternalServerError)
			return
		}
	}
}

// GetBalanceHandler возвращает текущий баланс пользователя.
func GetBalanceHandler(bs balanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := middleware.UserIDFromContext(r.Context())
		if err != nil {
			http.Error(w, unauthorizedError, http.StatusUnauthorized)
			return
		}

		b, err := bs.GetBalance(r.Context(), userID)
		if err != nil {
			logging.FromContext(r.Context()).Error("get balance error", slog.Any("err", err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resp := dto.BalanceResponse{
			Current:   float64(b.Accrual-b.Withdrawn) / model.AccrualScale,
			Withdrawn: float64(b.Withdrawn) / model.AccrualScale,
		}
		if err = json.NewEncoder(w).Encode(resp); err != nil {
			logging.FromContext(r.Context()).Error("get balance encode error", slog.Any("err", err))
			http.Error(w, "cannot encode response", http.StatusInternalServerError)
			return
		}
	}
}

// WithdrawHandler регистрирует списание баллов лояльности в счет оплаты нового заказа.
func WithdrawHandler(bs balanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := middleware.UserIDFromContext(r.Context())
		if err != nil {
			http.Error(w, unauthorizedError, http.StatusUnauthorized)
			return
		}

		var body dto.WithdrawRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "cannot decode request body", http.StatusBadRequest)
			return
		}

		sum, err := model.AccrualMajorToMinor(body.Sum)
		if err != nil {
			http.Error(w, "invalid withdrawal sum", http.StatusBadRequest)
			return
		}

		err = bs.Withdraw(r.Context(), userID, body.Order, sum)
		if err == nil {
			return
		}
		if errors.Is(err, model.ErrInvalidOrderNumber) {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		if errors.Is(err, model.ErrInsufficientFunds) {
			http.Error(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		if errors.Is(err, model.ErrWithdrawConflict) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if errors.Is(err, model.ErrInvalidWithdrawalSum) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		logging.FromContext(r.Context()).Error("withdraw error", slog.Any("err", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// GetWithdrawalsHandler возвращает информацию о выводе средств.
//
// При отсутствии информации возвращает 204 No Content.
func GetWithdrawalsHandler(bs balanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := middleware.UserIDFromContext(r.Context())
		if err != nil {
			http.Error(w, unauthorizedError, http.StatusUnauthorized)
			return
		}

		ws, err := bs.ListWithdrawals(r.Context(), userID)
		if err != nil {
			logging.FromContext(r.Context()).Error("get withdrawals error", slog.Any("err", err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if len(ws) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		resp := make([]dto.WithdrawalResponse, 0, len(ws))
		for _, w0 := range ws {
			resp = append(resp, dto.ToWithdrawalResponse(w0))
		}

		w.Header().Set("Content-Type", "application/json")
		if err = json.NewEncoder(w).Encode(resp); err != nil {
			logging.FromContext(r.Context()).Error("get withdrawals encode error", slog.Any("err", err))
			http.Error(w, "cannot encode response", http.StatusInternalServerError)
			return
		}
	}
}
