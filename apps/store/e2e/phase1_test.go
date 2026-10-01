//go:build integration

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
	restapi "github.com/Clint-Mathews/chaosbox/apps/store/rest-api"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPhase1Flow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("chaosbox"),
		postgres.WithUsername("chaosbox"),
		postgres.WithPassword("chaosbox"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(time.Minute),
		),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Errorf("terminate PostgreSQL container: %v", err)
		}
	})

	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}

	db := openPreparedDatabase(t, ctx, databaseURL)
	if err := db.Prepare(ctx); err != nil {
		t.Fatalf("prepare database a second time: %v", err)
	}

	inspectionPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create inspection pool: %v", err)
	}
	t.Cleanup(inspectionPool.Close)

	assertPreparedSchema(t, ctx, inspectionPool)

	server := httptest.NewServer(restapi.NewHandler(db))
	client := server.Client()

	healthBody := request(t, client, http.MethodGet, server.URL+"/health", nil, http.StatusOK)
	var health map[string]string
	decodeJSON(t, healthBody, &health)
	if health["status"] != "ok" {
		t.Fatalf("expected healthy response, got %s", healthBody)
	}

	productsBody := request(t, client, http.MethodGet, server.URL+"/products", nil, http.StatusOK)
	var products []database.Product
	decodeJSON(t, productsBody, &products)
	if len(products) != 5 {
		t.Fatalf("expected five seeded products, got %d", len(products))
	}
	assertProductNames(t, products)

	filteredBody := request(t, client, http.MethodGet, server.URL+"/products?name_prefix=Wire", nil, http.StatusOK)
	var filteredProducts []database.Product
	decodeJSON(t, filteredBody, &filteredProducts)
	if len(filteredProducts) != 1 || filteredProducts[0].Name != "Wireless Mouse" {
		t.Fatalf("unexpected prefix results: %+v", filteredProducts)
	}

	createBody := []byte(`{"items":[{"product_id":1,"quantity":1},{"product_id":2,"quantity":2}]}`)
	createdBody := request(t, client, http.MethodPost, server.URL+"/orders", createBody, http.StatusCreated)
	var createdOrder database.Order
	decodeJSON(t, createdBody, &createdOrder)
	if !regexp.MustCompile(`^ORD-[0-9A-F]{16}$`).MatchString(createdOrder.OrderNumber) {
		t.Fatalf("unexpected order number %q", createdOrder.OrderNumber)
	}
	if len(createdOrder.Items) != 2 || createdOrder.Items[0].ProductID != 1 || createdOrder.Items[1].Quantity != 2 {
		t.Fatalf("unexpected created order: %+v", createdOrder)
	}

	orderBody := request(t, client, http.MethodGet, server.URL+"/orders/"+createdOrder.OrderNumber, nil, http.StatusOK)
	var order database.Order
	decodeJSON(t, orderBody, &order)
	if order.OrderNumber != createdOrder.OrderNumber {
		t.Fatalf("unexpected order: %+v", order)
	}
	request(t, client, http.MethodGet, server.URL+"/orders/ORD-MISSING", nil, http.StatusNotFound)

	invalidBody := []byte(`{"items":[{"product_id":999999,"quantity":1}]}`)
	request(t, client, http.MethodPost, server.URL+"/orders", invalidBody, http.StatusBadRequest)
	assertRowCount(t, ctx, inspectionPool, "orders", 1)

	if _, err := db.CreateOrder(ctx, []database.NewOrderItem{
		{ProductID: 1, Quantity: 1},
		{ProductID: 999999, Quantity: 1},
	}); !errors.Is(err, database.ErrProductNotFound) {
		t.Fatalf("expected missing product error, got %v", err)
	}
	assertRowCount(t, ctx, inspectionPool, "orders", 1)

	if _, err := db.CreateOrder(ctx, []database.NewOrderItem{
		{ProductID: 1, Quantity: 1},
		{ProductID: 1, Quantity: 2},
	}); !errors.Is(err, database.ErrDuplicateProduct) {
		t.Fatalf("expected duplicate product error, got %v", err)
	}
	assertRowCount(t, ctx, inspectionPool, "orders", 1)

	if _, err := db.CreateOrder(ctx, []database.NewOrderItem{{ProductID: 1, Quantity: 0}}); err == nil {
		t.Fatal("expected zero quantity to fail")
	}
	assertRowCount(t, ctx, inspectionPool, "orders", 1)

	server.Close()
	db.Close()

	db = openPreparedDatabase(t, ctx, databaseURL)
	server = httptest.NewServer(restapi.NewHandler(db))
	t.Cleanup(server.Close)
	t.Cleanup(db.Close)

	persistedBody := request(t, server.Client(), http.MethodGet, server.URL+"/orders/"+createdOrder.OrderNumber, nil, http.StatusOK)
	var persistedOrder database.Order
	decodeJSON(t, persistedBody, &persistedOrder)
	if persistedOrder.OrderNumber != createdOrder.OrderNumber {
		t.Fatalf("order did not persist across restart: %+v", persistedOrder)
	}
	assertRowCount(t, ctx, inspectionPool, "products", 5)

	if _, err := inspectionPool.Exec(ctx,
		`INSERT INTO orders (order_number) VALUES ($1)`,
		createdOrder.OrderNumber,
	); postgresErrorCode(err) != "23505" {
		t.Fatalf("expected unique violation for duplicate order number, got %v", err)
	}

	if _, err := inspectionPool.Exec(ctx,
		`INSERT INTO order_items (order_id, product_id, quantity) VALUES ($1, $2, 0)`,
		createdOrder.ID,
		products[2].ID,
	); postgresErrorCode(err) != "23514" {
		t.Fatalf("expected check violation for zero quantity, got %v", err)
	}

	if _, err := inspectionPool.Exec(ctx,
		`DELETE FROM products WHERE id = $1`,
		products[0].ID,
	); postgresErrorCode(err) != "23503" {
		t.Fatalf("expected foreign-key violation deleting referenced product, got %v", err)
	}

	if _, err := inspectionPool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, createdOrder.ID); err != nil {
		t.Fatalf("delete order: %v", err)
	}
	var itemCount int
	if err := inspectionPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM order_items WHERE order_id = $1`,
		createdOrder.ID,
	).Scan(&itemCount); err != nil {
		t.Fatalf("count deleted order items: %v", err)
	}
	if itemCount != 0 {
		t.Fatalf("expected cascade to delete order items, got %d", itemCount)
	}

	if _, err := db.GetOrder(ctx, createdOrder.OrderNumber); !errors.Is(err, database.ErrOrderNotFound) {
		t.Fatalf("expected deleted order not to be found, got %v", err)
	}
}

func openPreparedDatabase(t *testing.T, ctx context.Context, databaseURL string) *database.DB {
	t.Helper()
	db, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Prepare(ctx); err != nil {
		db.Close()
		t.Fatalf("prepare database: %v", err)
	}
	return db
}

func assertPreparedSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	for _, table := range []string{"products", "orders", "order_items", "schema_migrations"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist", table)
		}
	}

	assertRowCount(t, ctx, pool, "schema_migrations", 1)
	assertRowCount(t, ctx, pool, "products", 5)

	var indexDefinition string
	if err := pool.QueryRow(ctx,
		`SELECT indexdef FROM pg_indexes WHERE indexname = 'products_name_prefix_idx'`,
	).Scan(&indexDefinition); err != nil {
		t.Fatalf("query prefix index: %v", err)
	}
	if !strings.Contains(indexDefinition, "text_pattern_ops") {
		t.Fatalf("expected text_pattern_ops index, got %q", indexDefinition)
	}
}

func assertProductNames(t *testing.T, products []database.Product) {
	t.Helper()
	want := map[string]bool{
		"Mechanical Keyboard":         true,
		"Wireless Mouse":              true,
		"USB-C Dock":                  true,
		"27-inch Monitor":             true,
		"Noise-Cancelling Headphones": true,
	}
	for _, product := range products {
		if !want[product.Name] {
			t.Fatalf("unexpected seeded product %q", product.Name)
		}
		delete(want, product.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing seeded products: %+v", want)
	}
}

func assertRowCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, want int) {
	t.Helper()
	allowedTables := map[string]bool{
		"orders":            true,
		"products":          true,
		"schema_migrations": true,
	}
	if !allowedTables[table] {
		t.Fatalf("row-count assertion does not allow table %q", table)
	}

	var got int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM `+table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("expected %d rows in %s, got %d", want, table, got)
	}
}

func request(t *testing.T, client *http.Client, method, url string, body []byte, wantStatus int) []byte {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("expected status %d, got %d: %s", wantStatus, response.StatusCode, responseBody)
	}
	if got := response.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected JSON content type, got %q", got)
	}

	return responseBody
}

func decodeJSON(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("decode JSON %s: %v", body, err)
	}
}

func postgresErrorCode(err error) string {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		return postgresError.Code
	}
	return ""
}
