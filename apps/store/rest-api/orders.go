package restapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
)

type createOrderRequest struct {
	Items []database.NewOrderItem `json:"items"`
}

func (h handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var request createOrderRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request body must contain one JSON object")
		return
	}

	if message := validateOrderItems(request.Items); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}

	order, err := h.store.CreateOrder(r.Context(), request.Items)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrProductNotFound):
			writeError(w, http.StatusBadRequest, "one or more products do not exist")
		case errors.Is(err, database.ErrDuplicateProduct):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("create order: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create order")
		}
		return
	}

	writeJSON(w, http.StatusCreated, order)
}

func (h handler) listOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.store.ListOrders(r.Context())
	if err != nil {
		log.Printf("list orders: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch orders")
		return
	}

	writeJSON(w, http.StatusOK, orders)
}

func validateOrderItems(items []database.NewOrderItem) string {
	if len(items) == 0 {
		return "an order must contain at least one item"
	}

	products := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.ProductID <= 0 {
			return "product_id must be a positive integer"
		}
		if item.Quantity <= 0 {
			return "quantity must be a positive integer"
		}
		if _, exists := products[item.ProductID]; exists {
			return database.ErrDuplicateProduct.Error()
		}
		products[item.ProductID] = struct{}{}
	}

	return ""
}
