package restapi

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const requestIDHeader = "X-Request-ID"

type loggerContextKey struct{}
type requestIDContextKey struct{}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += n
	return n, err
}

func (w *responseRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func requestLogging(logger *slog.Logger, next http.Handler, metricSets ...*httpMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var metrics *httpMetrics
		if len(metricSets) > 0 {
			metrics = metricSets[0]
			metrics.inFlight.Inc()
			defer metrics.inFlight.Dec()
		}
		requestID := r.Header.Get(requestIDHeader)
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}

		requestLogger := logger.With("request_id", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		ctx = context.WithValue(ctx, loggerContextKey{}, requestLogger)
		r = r.WithContext(ctx)
		w.Header().Set(requestIDHeader, requestID)
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}

		route := routePattern(r.Pattern)
		level := slog.LevelInfo
		if route == "/health" || route == "/metrics" {
			level = slog.LevelDebug
		}
		if metrics != nil {
			metrics.observe(r.Method, route, recorder.status, time.Since(started))
		}
		requestLogger.Log(r.Context(), level, "request completed",
			"method", r.Method,
			"route", route,
			"status", recorder.status,
			"duration_ms", float64(time.Since(started).Microseconds())/1000,
			"response_bytes", recorder.bytes,
		)
	})
}

func routePattern(pattern string) string {
	if pattern == "" {
		return "unmatched"
	}
	if _, route, found := strings.Cut(pattern, " "); found {
		return route
	}
	return pattern
}

func validRequestID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i := range len(value) {
		c := value[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' && c != '.' {
			return false
		}
	}
	return true
}

func newRequestID() string {
	return rand.Text()
}

func loggerFromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return logger
}

func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}
