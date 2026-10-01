package restapi

import (
	"context"
	"net/http"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
)

func (h handler) listProducts(w http.ResponseWriter, r *http.Request) {
	namePrefix := r.URL.Query().Get("name_prefix")
	products, err := h.listProductsCached(r.Context(), namePrefix)
	if err != nil {
		loggerFromContext(r.Context()).ErrorContext(r.Context(), "list products failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "failed to fetch products")
		return
	}

	writeJSON(w, r, http.StatusOK, products)
}

func (h handler) listProductsCached(ctx context.Context, namePrefix string) ([]database.Product, error) {
	if namePrefix != "" {
		return h.store.ListProducts(ctx, namePrefix)
	}

	h.productCache.mu.RLock()
	if h.productCache.loaded {
		products := h.productCache.products
		h.productCache.mu.RUnlock()
		return products, nil
	}
	h.productCache.mu.RUnlock()

	h.productCache.mu.Lock()
	defer h.productCache.mu.Unlock()
	if h.productCache.loaded {
		return h.productCache.products, nil
	}

	products, err := h.store.ListProducts(ctx, "")
	if err != nil {
		return nil, err
	}
	h.productCache.products = make([]database.Product, len(products))
	copy(h.productCache.products, products)
	h.productCache.loaded = true
	return h.productCache.products, nil
}
