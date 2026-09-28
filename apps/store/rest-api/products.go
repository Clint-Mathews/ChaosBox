package restapi

import (
	"net/http"
)

func (h handler) listProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.store.ListProducts(r.Context(), r.URL.Query().Get("name_prefix"))
	if err != nil {
		loggerFromContext(r.Context()).ErrorContext(r.Context(), "list products failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "failed to fetch products")
		return
	}

	writeJSON(w, r, http.StatusOK, products)
}
