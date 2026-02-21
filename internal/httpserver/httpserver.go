// Package httpserver содержит компонент HTTP-сервера приложения и маршрутизацию API.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"zerogravity-82/gophermart/internal/auth"
	"zerogravity-82/gophermart/internal/httpserver/handler"
	"zerogravity-82/gophermart/internal/httpserver/middleware"
	"zerogravity-82/gophermart/internal/logging"
	"zerogravity-82/gophermart/internal/service"
)

// HTTPServer поднимает HTTP-сервер и настраивает роутинг и middleware.
//
// Экземпляр содержит ссылки на сервисы доменной логики и менеджер JWT.
type HTTPServer struct {
	addr string
	jwtm *auth.JWTManager
	us   *service.UserService
	os   *service.OrderService
	bs   *service.BalanceService
}

// New создает HTTPServer.
//
// addr — адрес, на котором слушает HTTP-сервер (например, "localhost:8080").
func New(
	addr string,
	jwtm *auth.JWTManager,
	us *service.UserService,
	os *service.OrderService,
	bs *service.BalanceService,
) *HTTPServer {
	return &HTTPServer{addr: addr, jwtm: jwtm, us: us, os: os, bs: bs}
}

// Run запускает HTTP-сервер и блокируется, пока не отменен ctx или сервер не остановится с ошибкой.
func (s *HTTPServer) Run(ctx context.Context) error {
	const shutdownTimeout = 10 * time.Second

	logger := logging.FromContext(ctx)

	srv := &http.Server{
		Addr:              s.addr,
		Handler:           s.buildRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP server", slog.String("addr", s.addr))
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		err := srv.Shutdown(shutdownCtx)
		if err == nil {
			logger.Info(
				"stopped HTTP server",
				slog.String("addr", s.addr),
				slog.String("reason", "gracefully shutdown"),
			)
			return nil
		}
		logger.Error(
			"stopped HTTP server",
			slog.String("addr", s.addr),
			slog.Any("err", err),
		)
		return fmt.Errorf("stopped HTTP server: %w", err)
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			logger.Info(
				"stopped HTTP server",
				slog.String("addr", s.addr),
				slog.String("reason", "closed"),
			)
			return nil
		}
		logger.Error(
			"failed HTTP server",
			slog.String("addr", s.addr),
			slog.Any("err", err),
		)
		return fmt.Errorf("failed HTTP server %w", err)
	}
}

func (s *HTTPServer) buildRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(
		chimw.RequestID,
		middleware.RequestIDLogger(),
		chimw.RealIP,
		chimw.Recoverer,
		chimw.StripSlashes,
		middleware.AccessLogger(),
		chimw.Compress(5),
	)

	r.Route("/api/user", func(r chi.Router) {
		applicationJSONContentType := chimw.AllowContentType("application/json")
		r.With(applicationJSONContentType).Post("/register", handler.RegisterHandler(s.us))
		r.With(applicationJSONContentType).Post("/login", handler.LoginHandler(s.us))
		r.With(applicationJSONContentType).Post("/refresh", handler.RefreshHandler(s.us))
		r.With(applicationJSONContentType).Post("/logout", handler.LogoutHandler(s.us))

		textPlainContentType := chimw.AllowContentType("text/plain")
		r.Group(func(r chi.Router) {
			r.Use(middleware.WithAuth(s.jwtm))
			r.With(textPlainContentType).Post("/orders", handler.UploadOrderHandler(s.os))
			r.Get("/orders", handler.GetOrdersHandler(s.os))
			r.Get("/balance", handler.GetBalanceHandler(s.bs))
			r.With(applicationJSONContentType).Post("/balance/withdraw", handler.WithdrawHandler(s.bs))
			r.Get("/withdrawals", handler.GetWithdrawalsHandler(s.bs))
		})
	})

	return r
}
