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

type requestContextKey struct{}

type requestContext struct {
	logger    *slog.Logger
	requestID string
}

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
		w.status = http.StatusOK
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

		ctx := context.WithValue(r.Context(), requestContextKey{}, requestContext{
			logger:    logger,
			requestID: requestID,
		})
		r = r.WithContext(ctx)
		w.Header().Set(requestIDHeader, requestID)
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}

		duration := time.Since(started)
		route := routePattern(r.Pattern)
		if metrics != nil {
			metrics.observe(r.Method, route, recorder.status, duration)
		}
		level := completionLogLevel(route, recorder.status, duration)
		if !logger.Enabled(r.Context(), level) {
			return
		}
		logger.LogAttrs(r.Context(), level, "request completed",
			slog.String("request_id", requestID),
			slog.String("method", r.Method),
			slog.String("route", route),
			slog.Int("status", recorder.status),
			slog.Float64("duration_ms", float64(duration.Microseconds())/1000),
			slog.Int("response_bytes", recorder.bytes),
		)
	})
}

func completionLogLevel(route string, status int, duration time.Duration) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	case duration >= slowRequestThreshold(route):
		return slog.LevelWarn
	default:
		return slog.LevelDebug
	}
}

func slowRequestThreshold(route string) time.Duration {
	if route == "/products" {
		return 500 * time.Millisecond
	}
	return time.Second
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

func logError(ctx context.Context, message string, args ...any) {
	request, ok := ctx.Value(requestContextKey{}).(requestContext)
	if !ok {
		slog.Default().ErrorContext(ctx, message, args...)
		return
	}
	request.logger.ErrorContext(ctx, message, append([]any{"request_id", request.requestID}, args...)...)
}

func RequestIDFromContext(ctx context.Context) string {
	request, _ := ctx.Value(requestContextKey{}).(requestContext)
	return request.requestID
}
