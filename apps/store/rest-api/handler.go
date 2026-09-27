package restapi

import (
	"context"
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

func NewHandler(store Store) http.Handler {
	handler := handler{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /products", handler.listProducts)
	mux.HandleFunc("POST /orders", handler.createOrder)
	mux.HandleFunc("GET /orders", handler.listOrders)

	return mux
}
