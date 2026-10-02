package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
		preserve bool
	}{
		{name: "generated"},
		{name: "valid", incoming: "load-test_123.example", preserve: true},
		{name: "spaces rejected", incoming: "unsafe request id"},
		{name: "non-ASCII rejected", incoming: "requ\xc3\xaate"},
		{name: "too long rejected", incoming: strings.Repeat("a", 129)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var contextID string
			store := fakeStore{listProducts: func(ctx context.Context, _ string) ([]database.Product, error) {
				contextID = RequestIDFromContext(ctx)
				return nil, nil
			}}
			request := httptest.NewRequest(http.MethodGet, "/products", nil)
			if test.incoming != "" {
				request.Header.Set(requestIDHeader, test.incoming)
			}
			recorder := httptest.NewRecorder()

			NewHandler(store).ServeHTTP(recorder, request)

			responseID := recorder.Header().Get(requestIDHeader)
			if !validRequestID(responseID) {
				t.Fatalf("expected a valid response request ID, got %q", responseID)
			}
			if test.preserve && responseID != test.incoming {
				t.Fatalf("expected request ID %q, got %q", test.incoming, responseID)
			}
			if !test.preserve && test.incoming != "" && responseID == test.incoming {
				t.Fatalf("expected unsafe request ID %q to be replaced", test.incoming)
			}
			if contextID != responseID {
				t.Fatalf("expected store context request ID %q, got %q", responseID, contextID)
			}
		})
	}
}

func TestRequestCompletionLog(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})
	mux := http.NewServeMux()
	mux.Handle("POST /orders", handler)
	request := httptest.NewRequest(http.MethodPost, "/orders", nil)
	request.Header.Set(requestIDHeader, "request-123")
	recorder := httptest.NewRecorder()

	requestLogging(logger, mux).ServeHTTP(recorder, request)

	entry := decodeLogEntry(t, output.String())
	assertLogValue(t, entry, "msg", "request completed")
	assertLogValue(t, entry, "level", "DEBUG")
	assertLogValue(t, entry, "request_id", "request-123")
	assertLogValue(t, entry, "method", http.MethodPost)
	assertLogValue(t, entry, "route", "/orders")
	assertLogValue(t, entry, "status", float64(http.StatusCreated))
	assertLogValue(t, entry, "response_bytes", float64(len("created")))
	if _, ok := entry["duration_ms"].(float64); !ok {
		t.Fatalf("expected numeric duration_ms, got %#v", entry["duration_ms"])
	}
}

func TestRequestCompletionLogCapturesImplicitOK(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	request := httptest.NewRequest(http.MethodGet, "/implicit", nil)
	recorder := httptest.NewRecorder()

	requestLogging(logger, handler).ServeHTTP(recorder, request)

	entry := decodeLogEntry(t, output.String())
	assertLogValue(t, entry, "status", float64(http.StatusOK))
	assertLogValue(t, entry, "response_bytes", float64(len("ok")))
}

func TestSuccessfulCompletionLogIsSuppressedAtInfo(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	request := httptest.NewRequest(http.MethodGet, "/products", nil)
	recorder := httptest.NewRecorder()

	NewHandler(fakeStore{}, logger).ServeHTTP(recorder, request)

	if output.Len() != 0 {
		t.Fatalf("expected successful completion log to be suppressed, got %q", output.String())
	}
}

func TestCompletionLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		route    string
		status   int
		duration time.Duration
		want     slog.Level
	}{
		{name: "ordinary success", route: "/orders/{id}", status: http.StatusOK, duration: 100 * time.Millisecond, want: slog.LevelDebug},
		{name: "slow product", route: "/products", status: http.StatusOK, duration: 500 * time.Millisecond, want: slog.LevelWarn},
		{name: "slow order", route: "/orders", status: http.StatusCreated, duration: time.Second, want: slog.LevelWarn},
		{name: "client error", route: "/orders/{id}", status: http.StatusNotFound, want: slog.LevelWarn},
		{name: "server error", route: "/orders", status: http.StatusInternalServerError, want: slog.LevelError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := completionLogLevel(test.route, test.status, test.duration); got != test.want {
				t.Fatalf("expected level %s, got %s", test.want, got)
			}
		})
	}
}

func TestInternalErrorLogUsesRequestID(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	store := fakeStore{listProducts: func(context.Context, string) ([]database.Product, error) {
		return nil, errors.New("database unavailable")
	}}
	request := httptest.NewRequest(http.MethodGet, "/products", nil)
	request.Header.Set(requestIDHeader, "request-456")
	recorder := httptest.NewRecorder()

	NewHandler(store, logger).ServeHTTP(recorder, request)

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected internal error and completion logs, got %d: %s", len(lines), output.String())
	}
	for _, line := range lines {
		entry := decodeLogEntry(t, line)
		assertLogValue(t, entry, "request_id", "request-456")
	}
}

func TestErrorRouteLogIsStable(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		status int
		route  string
	}{
		{name: "order not found", method: http.MethodGet, target: "/orders/123?token=secret", status: http.StatusNotFound, route: "/orders/{id}"},
		{name: "method not allowed", method: http.MethodPatch, target: "/orders", status: http.StatusMethodNotAllowed, route: "unmatched"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			request := httptest.NewRequest(test.method, test.target, nil)
			recorder := httptest.NewRecorder()

			NewHandler(fakeStore{}, logger).ServeHTTP(recorder, request)

			entry := decodeLogEntry(t, output.String())
			assertLogValue(t, entry, "route", test.route)
			assertLogValue(t, entry, "status", float64(test.status))
		})
	}
}

func TestHealthCompletionLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		level     slog.Level
		wantEntry bool
	}{
		{name: "suppressed at info", level: slog.LevelInfo},
		{name: "included at debug", level: slog.LevelDebug, wantEntry: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: test.level}))
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			recorder := httptest.NewRecorder()

			NewHandler(nil, logger).ServeHTTP(recorder, request)

			if got := output.Len() > 0; got != test.wantEntry {
				t.Fatalf("expected log entry=%t, got output %q", test.wantEntry, output.String())
			}
			if test.wantEntry {
				entry := decodeLogEntry(t, output.String())
				assertLogValue(t, entry, "level", "DEBUG")
				assertLogValue(t, entry, "route", "/health")
			}
		})
	}
}

func decodeLogEntry(t *testing.T, line string) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("decode log entry: %v: %s", err, line)
	}
	return entry
}

func assertLogValue(t *testing.T, entry map[string]any, key string, want any) {
	t.Helper()
	if got := entry[key]; got != want {
		t.Fatalf("expected %s %#v, got %#v", key, want, got)
	}
}
