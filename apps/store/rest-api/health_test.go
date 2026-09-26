package restapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if got, want := recorder.Header().Get("Content-Type"), "application/json"; got != want {
		t.Fatalf("expected Content-Type %q, got %q", want, got)
	}

	if got, want := recorder.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}
}

func TestHealthHandlerRejectsOtherMethods(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/health", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, recorder.Code)
	}

	if got, want := recorder.Header().Get("Allow"), "GET, HEAD"; got != want {
		t.Fatalf("expected Allow header %q, got %q", want, got)
	}
}
