// Package http provides HTTP server and routing functionality.
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"gophermart/internal/http/handlers"
	"gophermart/internal/http/middleware"
)

// RouterDeps holds the dependencies for the router.
type RouterDeps struct {
	AuthService    handlers.AuthService
	OrdersService  handlers.OrdersService
	BalanceService handlers.BalanceService
}

// NewRouter creates a new HTTP router with all routes configured.
func NewRouter(deps *RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.GzipDecompress)
	r.Use(middleware.GzipCompress)
	r.Use(middleware.LimitRequestBody(1 << 20))
	r.Use(chiMiddleware.StripSlashes)

	userHandler := handlers.NewUserHandler(deps.AuthService)
	ordersHandler := handlers.NewOrdersHandler(deps.OrdersService)
	balanceHandler := handlers.NewBalanceHandler(deps.BalanceService)

	r.Post("/api/user/register", userHandler.Register)
	r.Post("/api/user/login", userHandler.Login)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(deps.AuthService))

		r.Post("/api/user/orders", ordersHandler.Upload)
		r.Get("/api/user/orders", ordersHandler.List)
		r.Get("/api/user/balance", balanceHandler.Get)
		r.Post("/api/user/balance/withdraw", balanceHandler.Withdraw)
		r.Get("/api/user/withdrawals", balanceHandler.Withdrawals)
	})

	return r
}
