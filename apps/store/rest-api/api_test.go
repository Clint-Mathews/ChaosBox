package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	return store.listProducts(ctx, prefix)
}

func (store fakeStore) CreateOrder(ctx context.Context, items []database.NewOrderItem) (database.Order, error) {
	return store.createOrder(ctx, items)
}

func (store fakeStore) ListOrders(ctx context.Context) ([]database.Order, error) {
	return store.listOrders(ctx)
}

func TestListProducts(t *testing.T) {
	createdAt := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	store := fakeStore{
		listProducts: func(_ context.Context, prefix string) ([]database.Product, error) {
			if prefix != "Wire" {
				t.Fatalf("expected prefix %q, got %q", "Wire", prefix)
			}
			return []database.Product{{ID: 2, Name: "Wireless Mouse", CreatedAt: createdAt}}, nil
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/products?name_prefix=Wire", nil)

	NewHandler(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var products []database.Product
	if err := json.NewDecoder(recorder.Body).Decode(&products); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(products) != 1 || products[0].Name != "Wireless Mouse" {
		t.Fatalf("unexpected products: %+v", products)
	}
}

func TestCreateOrder(t *testing.T) {
	createdAt := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	store := fakeStore{
		createOrder: func(_ context.Context, items []database.NewOrderItem) (database.Order, error) {
			if len(items) != 2 || items[0].ProductID != 1 || items[1].Quantity != 2 {
				t.Fatalf("unexpected items: %+v", items)
			}
			return database.Order{
				ID:          7,
				OrderNumber: "ORD-123",
				Items: []database.OrderItem{
					{ProductID: 1, ProductName: "Mechanical Keyboard", Quantity: 1},
					{ProductID: 2, ProductName: "Wireless Mouse", Quantity: 2},
				},
				CreatedAt: createdAt,
			}, nil
		},
	}
	body := bytes.NewBufferString(`{"items":[{"product_id":1,"quantity":1},{"product_id":2,"quantity":2}]}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/orders", body)

	NewHandler(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}
	var order database.Order
	if err := json.NewDecoder(recorder.Body).Decode(&order); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if order.ID != 7 || len(order.Items) != 2 {
		t.Fatalf("unexpected order: %+v", order)
	}
}

func TestCreateOrderRejectsDuplicateProducts(t *testing.T) {
	called := false
	store := fakeStore{
		createOrder: func(_ context.Context, _ []database.NewOrderItem) (database.Order, error) {
			called = true
			return database.Order{}, nil
		},
	}
	body := bytes.NewBufferString(`{"items":[{"product_id":1,"quantity":1},{"product_id":1,"quantity":2}]}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/orders", body)

	NewHandler(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	if called {
		t.Fatal("expected invalid order not to reach the store")
	}
}

func TestListOrders(t *testing.T) {
	store := fakeStore{
		listOrders: func(_ context.Context) ([]database.Order, error) {
			return []database.Order{{ID: 7, OrderNumber: "ORD-123", Items: []database.OrderItem{}}}, nil
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
	if len(orders) != 1 || orders[0].OrderNumber != "ORD-123" {
		t.Fatalf("unexpected orders: %+v", orders)
	}
}
