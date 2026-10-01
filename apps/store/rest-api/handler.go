package restapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Store interface {
	ListProducts(context.Context, string) ([]database.Product, error)
	CreateOrder(context.Context, []database.NewOrderItem) (database.Order, error)
	GetOrder(context.Context, string) (database.Order, error)
}

type handler struct {
	store Store
}

func NewHandler(store Store, loggers ...*slog.Logger) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if source, ok := store.(database.PoolStatsSource); ok {
		registry.MustRegister(database.NewPoolCollector(source))
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	return NewInstrumentedHandler(store, logger, registry)
}

func NewInstrumentedHandler(store Store, logger *slog.Logger, registry *prometheus.Registry) http.Handler {
	handler := handler{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /products", handler.listProducts)
	mux.HandleFunc("POST /orders", handler.createOrder)
	mux.HandleFunc("GET /orders/{order_number}", handler.getOrder)

	return requestLogging(logger, mux, newHTTPMetrics(registry))
}
