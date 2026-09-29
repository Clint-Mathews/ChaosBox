package restapi

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type httpMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	inFlight        prometheus.Gauge
}

func newHTTPMetrics(registerer prometheus.Registerer) *httpMetrics {
	metrics := &httpMetrics{
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "store",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of completed HTTP requests.",
		}, []string{"method", "route", "status"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "store",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"method", "route", "status"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "store",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Current number of HTTP requests being served.",
		}),
	}
	registerer.MustRegister(metrics.requestsTotal, metrics.requestDuration, metrics.inFlight)
	return metrics
}

func (metrics *httpMetrics) observe(method, route string, status int, duration time.Duration) {
	labels := []string{method, route, strconv.Itoa(status)}
	metrics.requestsTotal.WithLabelValues(labels...).Inc()
	metrics.requestDuration.WithLabelValues(labels...).Observe(duration.Seconds())
}
