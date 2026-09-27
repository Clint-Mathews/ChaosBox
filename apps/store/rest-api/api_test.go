package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
)

type fakeStore struct {
	listProducts func(context.Context, string) ([]database.Product, error)
	createOrder  func(context.Context, []database.NewOrderItem) (database.Order, error)
	listOrders   func(context.Context) ([]database.Order, error)
}

func (store fakeStore) ListProducts(ctx context.Context, prefix string) ([]database.Product, error) {
	if store.listProducts == nil {
		return []database.Product{}, nil
	}
	return store.listProducts(ctx, prefix)
}

func (store fakeStore) CreateOrder(ctx context.Context, items []database.NewOrderItem) (database.Order, error) {
	if store.createOrder == nil {
		return database.Order{}, nil
	}
	return store.createOrder(ctx, items)
}

func (store fakeStore) ListOrders(ctx context.Context) ([]database.Order, error) {
	if store.listOrders == nil {
		return []database.Order{}, nil
	}
	return store.listOrders(ctx)
}

func TestListProducts(t *testing.T) {
	createdAt := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	want := []database.Product{{ID: 2, Name: "Wireless Mouse", CreatedAt: createdAt}}
	type contextKey string
	const key contextKey = "request"

	store := fakeStore{
		listProducts: func(ctx context.Context, prefix string) ([]database.Product, error) {
			if got := ctx.Value(key); got != "products" {
				t.Fatalf("expected request context value, got %v", got)
			}
			if prefix != "Wire" {
				t.Fatalf("expected prefix %q, got %q", "Wire", prefix)
			}
			return want, nil
		},
	}
	request := httptest.NewRequest(http.MethodGet, "/products?name_prefix=Wire", nil)
	request = request.WithContext(context.WithValue(request.Context(), key, "products"))
	recorder := httptest.NewRecorder()

	NewHandler(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}
	var products []database.Product
	if err := json.NewDecoder(recorder.Body).Decode(&products); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !reflect.DeepEqual(products, want) {
		t.Fatalf("expected products %+v, got %+v", want, products)
	}
}

func TestListProductsReturnsEmptyArray(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/products", nil)

	NewHandler(fakeStore{}).ServeHTTP(recorder, request)

	if got, want := recorder.Body.String(), "[]\n"; got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}
}

func TestListProductsHandlesStoreError(t *testing.T) {
	store := fakeStore{
		listProducts: func(context.Context, string) ([]database.Product, error) {
			return nil, errors.New("database unavailable")
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/products", nil)

	NewHandler(store).ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusInternalServerError, "failed to fetch products")
}

func TestCreateOrder(t *testing.T) {
	createdAt := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	wantItems := []database.NewOrderItem{{ProductID: 1, Quantity: 1}, {ProductID: 2, Quantity: 2}}
	wantOrder := database.Order{
		ID:          7,
		OrderNumber: "ORD-123",
		Items: []database.OrderItem{
			{ProductID: 1, ProductName: "Mechanical Keyboard", Quantity: 1},
			{ProductID: 2, ProductName: "Wireless Mouse", Quantity: 2},
		},
		CreatedAt: createdAt,
	}
	store := fakeStore{
		createOrder: func(_ context.Context, items []database.NewOrderItem) (database.Order, error) {
			if !reflect.DeepEqual(items, wantItems) {
				t.Fatalf("expected items %+v, got %+v", wantItems, items)
			}
			return wantOrder, nil
		},
	}
	body := bytes.NewBufferString(`{"items":[{"product_id":1,"quantity":1},{"product_id":2,"quantity":2}]}   `)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/orders", body)

	NewHandler(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}
	var order database.Order
	if err := json.NewDecoder(recorder.Body).Decode(&order); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("expected order %+v, got %+v", wantOrder, order)
	}
}

func TestCreateOrderRejectsInvalidJSON(t *testing.T) {
	oversized := `{"items":[],"padding":"` + strings.Repeat("a", 1<<20) + `"}`
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "malformed", body: `{"items":[`},
		{name: "array", body: `[]`},
		{name: "wrong items type", body: `{"items":"invalid"}`},
		{name: "unknown top-level field", body: `{"items":[],"unknown":true}`},
		{name: "unknown item field", body: `{"items":[{"product_id":1,"quantity":1,"unknown":true}]}`},
		{name: "oversized", body: oversized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			store := fakeStore{
				createOrder: func(context.Context, []database.NewOrderItem) (database.Order, error) {
					called = true
					return database.Order{}, nil
				},
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(test.body))

			NewHandler(store).ServeHTTP(recorder, request)

			assertErrorResponse(t, recorder, http.StatusBadRequest, "invalid request body")
			if called {
				t.Fatal("expected invalid request not to reach the store")
			}
		})
	}
}

func TestCreateOrderRejectsTrailingJSON(t *testing.T) {
	tests := []string{
		`{"items":[{"product_id":1,"quantity":1}]} {}`,
		`{"items":[{"product_id":1,"quantity":1}]} trailing`,
	}

	for _, body := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))

		NewHandler(fakeStore{}).ServeHTTP(recorder, request)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "request body must contain one JSON object")
	}
}

func TestCreateOrderValidation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		message string
	}{
		{name: "null body", body: `null`, message: "an order must contain at least one item"},
		{name: "missing items", body: `{}`, message: "an order must contain at least one item"},
		{name: "null items", body: `{"items":null}`, message: "an order must contain at least one item"},
		{name: "empty items", body: `{"items":[]}`, message: "an order must contain at least one item"},
		{name: "zero product", body: `{"items":[{"product_id":0,"quantity":1}]}`, message: "product_id must be a positive integer"},
		{name: "negative product", body: `{"items":[{"product_id":-1,"quantity":1}]}`, message: "product_id must be a positive integer"},
		{name: "zero quantity", body: `{"items":[{"product_id":1,"quantity":0}]}`, message: "quantity must be a positive integer"},
		{name: "negative quantity", body: `{"items":[{"product_id":1,"quantity":-1}]}`, message: "quantity must be a positive integer"},
		{name: "duplicate product", body: `{"items":[{"product_id":1,"quantity":1},{"product_id":1,"quantity":2}]}`, message: database.ErrDuplicateProduct.Error()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			store := fakeStore{
				createOrder: func(context.Context, []database.NewOrderItem) (database.Order, error) {
					called = true
					return database.Order{}, nil
				},
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(test.body))

			NewHandler(store).ServeHTTP(recorder, request)

			assertErrorResponse(t, recorder, http.StatusBadRequest, test.message)
			if called {
				t.Fatal("expected invalid order not to reach the store")
			}
		})
	}
}

func TestCreateOrderMapsStoreErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "missing product", err: database.ErrProductNotFound, status: http.StatusBadRequest, message: "one or more products do not exist"},
		{name: "wrapped missing product", err: fmt.Errorf("insert: %w", database.ErrProductNotFound), status: http.StatusBadRequest, message: "one or more products do not exist"},
		{name: "duplicate product", err: database.ErrDuplicateProduct, status: http.StatusBadRequest, message: database.ErrDuplicateProduct.Error()},
		{name: "wrapped duplicate product", err: fmt.Errorf("insert: %w", database.ErrDuplicateProduct), status: http.StatusBadRequest, message: database.ErrDuplicateProduct.Error()},
		{name: "internal error", err: errors.New("password exposed internally"), status: http.StatusInternalServerError, message: "failed to create order"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := fakeStore{
				createOrder: func(context.Context, []database.NewOrderItem) (database.Order, error) {
					return database.Order{}, test.err
				},
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"items":[{"product_id":1,"quantity":1}]}`))

			NewHandler(store).ServeHTTP(recorder, request)

			assertErrorResponse(t, recorder, test.status, test.message)
			if strings.Contains(recorder.Body.String(), "password exposed") {
				t.Fatal("response exposed internal error details")
			}
		})
	}
}

func TestListOrders(t *testing.T) {
	want := []database.Order{{
		ID:          7,
		OrderNumber: "ORD-123",
		Items:       []database.OrderItem{{ProductID: 1, ProductName: "Mechanical Keyboard", Quantity: 2}},
	}}
	store := fakeStore{
		listOrders: func(context.Context) ([]database.Order, error) {
			return want, nil
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/orders", nil)

	NewHandler(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var orders []database.Order
	if err := json.NewDecoder(recorder.Body).Decode(&orders); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !reflect.DeepEqual(orders, want) {
		t.Fatalf("expected orders %+v, got %+v", want, orders)
	}
}

func TestListOrdersReturnsEmptyArray(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/orders", nil)

	NewHandler(fakeStore{}).ServeHTTP(recorder, request)

	if got, want := recorder.Body.String(), "[]\n"; got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}
}

func TestListOrdersHandlesStoreError(t *testing.T) {
	store := fakeStore{
		listOrders: func(context.Context) ([]database.Order, error) {
			return nil, errors.New("database unavailable")
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/orders", nil)

	NewHandler(store).ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusInternalServerError, "failed to fetch orders")
}

func TestHandlerRouting(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "unknown path", method: http.MethodGet, path: "/missing", status: http.StatusNotFound},
		{name: "products wrong method", method: http.MethodPost, path: "/products", status: http.StatusMethodNotAllowed},
		{name: "orders wrong method", method: http.MethodPut, path: "/orders", status: http.StatusMethodNotAllowed},
		{name: "products trailing slash", method: http.MethodGet, path: "/products/", status: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, nil)

			NewHandler(fakeStore{}).ServeHTTP(recorder, request)

			if recorder.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, recorder.Code)
			}
		})
	}
}

func assertErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}
	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if got := response["error"]; got != message {
		t.Fatalf("expected error %q, got %q", message, got)
	}
}
