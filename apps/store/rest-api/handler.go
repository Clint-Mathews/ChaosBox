package restapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
)

type Store interface {
	ListProducts(context.Context, string) ([]database.Product, error)
	CreateOrder(context.Context, []database.NewOrderItem) (database.Order, error)
	ListOrders(context.Context) ([]database.Order, error)
}

type handler struct {
	store Store
}

func NewHandler(store Store, loggers ...*slog.Logger) http.Handler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	handler := handler{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /products", handler.listProducts)
	mux.HandleFunc("POST /orders", handler.createOrder)
	mux.HandleFunc("GET /orders", handler.listOrders)

	return requestLogging(logger, mux)
}
