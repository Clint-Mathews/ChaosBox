package restapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Store interface {
	ListProducts(context.Context, string) ([]database.Product, error)
	CreateOrder(context.Context, []database.NewOrderItem) (database.Order, error)
	GetOrder(context.Context, int64) (database.Order, error)
}

type handler struct {
	store        Store
	productCache *productCache
}

type productCache struct {
	mu       sync.RWMutex
	loaded   bool
	response []byte
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
	handler := handler{store: store, productCache: &productCache{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /products", handler.listProducts)
	mux.HandleFunc("POST /orders", handler.createOrder)
	mux.HandleFunc("GET /orders/{id}", handler.getOrder)

	return requestLogging(logger, mux, newHTTPMetrics(registry))
}
