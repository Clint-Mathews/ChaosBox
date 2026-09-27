package restapi

import (
	"log"
	"net/http"
)

func (h handler) listProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.store.ListProducts(r.Context(), r.URL.Query().Get("name_prefix"))
	if err != nil {
		log.Printf("list products: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch products")
		return
	}

	writeJSON(w, http.StatusOK, products)
}
