package restapi

import (
	"context"
	"encoding/json"
	"net/http"
)

func (h handler) listProducts(w http.ResponseWriter, r *http.Request) {
	namePrefix := r.URL.Query().Get("name_prefix")
	if namePrefix != "" {
		products, err := h.store.ListProducts(r.Context(), namePrefix)
		if err != nil {
			logError(r.Context(), "list products failed", "error", err)
			writeError(w, r, http.StatusInternalServerError, "failed to fetch products")
			return
		}
		writeJSON(w, r, http.StatusOK, products)
		return
	}

	response, err := h.productResponseCached(r.Context())
	if err != nil {
		logError(r.Context(), "list products failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "failed to fetch products")
		return
	}
	writeJSONBytes(w, r, response)
}

func (h handler) productResponseCached(ctx context.Context) ([]byte, error) {
	h.productCache.mu.RLock()
	if h.productCache.loaded {
		response := h.productCache.response
		h.productCache.mu.RUnlock()
		return response, nil
	}
	h.productCache.mu.RUnlock()

	h.productCache.mu.Lock()
	defer h.productCache.mu.Unlock()
	if h.productCache.loaded {
		return h.productCache.response, nil
	}

	products, err := h.store.ListProducts(ctx, "")
	if err != nil {
		return nil, err
	}
	response, err := json.Marshal(products)
	if err != nil {
		return nil, err
	}
	h.productCache.response = append(response, '\n')
	h.productCache.loaded = true
	return h.productCache.response, nil
}
