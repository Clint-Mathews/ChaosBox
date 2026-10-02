package restapi

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logError(r.Context(), "encode response failed", "error", err)
	}
}

func writeJSONBytes(w http.ResponseWriter, r *http.Request, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(body); err != nil {
		logError(r.Context(), "write response failed", "error", err)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	writeJSON(w, r, status, map[string]string{"error": message})
}
