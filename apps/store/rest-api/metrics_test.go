package restapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTPMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	handler := NewInstrumentedHandler(fakeStore{}, discardLogger(), registry)

	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/products?name_prefix=Wire", nil),
		httptest.NewRequest(http.MethodGet, "/missing/one?secret=value", nil),
		httptest.NewRequest(http.MethodGet, "/missing/two", nil),
		httptest.NewRequest(http.MethodPatch, "/orders", nil),
	}
	for _, request := range requests {
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}

	want := `
# HELP store_http_requests_total Total number of completed HTTP requests.
# TYPE store_http_requests_total counter
store_http_requests_total{method="GET",route="/products",status="200"} 1
store_http_requests_total{method="GET",route="unmatched",status="404"} 2
store_http_requests_total{method="PATCH",route="unmatched",status="405"} 1
`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(want), "store_http_requests_total"); err != nil {
		t.Fatal(err)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	foundHistogram := false
	for _, family := range families {
		if family.GetName() == "store_http_request_duration_seconds" {
			foundHistogram = true
			if len(family.Metric) != 3 {
				t.Fatalf("expected 3 bounded histogram label sets, got %d", len(family.Metric))
			}
		}
	}
	if !foundHistogram {
		t.Fatal("expected HTTP duration histogram")
	}
}

func TestHTTPRequestsInFlight(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	store := fakeStore{listProducts: func(context.Context, string) ([]database.Product, error) {
		close(entered)
		<-release
		return nil, nil
	}}
	registry := prometheus.NewRegistry()
	handler := NewInstrumentedHandler(store, discardLogger(), registry)
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/products", nil))
		close(done)
	}()
	<-entered

	if got := metricValue(t, registry, "store_http_requests_in_flight"); got != 1 {
		t.Fatalf("expected one request in flight, got %v", got)
	}
	close(release)
	<-done
	if got := metricValue(t, registry, "store_http_requests_in_flight"); got != 0 {
		t.Fatalf("expected no requests in flight, got %v", got)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	handler := NewInstrumentedHandler(fakeStore{}, discardLogger(), registry)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/products", nil))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("expected Prometheus text content type, got %q", got)
	}
	for _, metric := range []string{"go_goroutines", "process_cpu_seconds_total", "store_http_requests_total", "store_http_request_duration_seconds", "store_http_requests_in_flight"} {
		if !strings.Contains(recorder.Body.String(), metric) {
			t.Fatalf("expected metrics response to contain %q", metric)
		}
	}
}

func metricValue(t *testing.T, registry *prometheus.Registry, name string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() == name {
			return family.Metric[0].GetGauge().GetValue()
		}
	}
	t.Fatalf("metric %q not found", name)
	return 0
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
