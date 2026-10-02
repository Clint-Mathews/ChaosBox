package restapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

type statusRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (recorder *statusRecorder) Header() http.Header {
	return recorder.header
}

func (recorder *statusRecorder) Write(body []byte) (int, error) {
	return recorder.body.Write(body)
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
}

func TestWriteJSONUsesImplicitOK(t *testing.T) {
	recorder := &statusRecorder{header: make(http.Header)}
	request := httptest.NewRequest(http.MethodGet, "/products", nil)

	writeJSON(recorder, request, http.StatusOK, map[string]string{"status": "ok"})

	if recorder.status != 0 {
		t.Fatalf("expected implicit status, got explicit status %d", recorder.status)
	}
	if got := recorder.body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestWriteJSONPreservesExplicitNonOKStatus(t *testing.T) {
	recorder := &statusRecorder{header: make(http.Header)}
	request := httptest.NewRequest(http.MethodPost, "/orders", nil)

	writeJSON(recorder, request, http.StatusCreated, map[string]int{"id": 1})

	if recorder.status != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.status)
	}
}
