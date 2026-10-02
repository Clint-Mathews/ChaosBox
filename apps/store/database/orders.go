package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrProductNotFound  = errors.New("product not found")
	ErrDuplicateProduct = errors.New("an order cannot contain duplicate products")
	ErrOrderNotFound    = errors.New("order not found")
)

func (db *DB) CreateOrder(ctx context.Context, items []NewOrderItem) (Order, error) {
	orderNumber, err := newOrderNumber()
	if err != nil {
		return Order{}, err
	}

	productIDs := make([]int64, len(items))
	quantities := make([]int64, len(items))
	for index, item := range items {
		productIDs[index] = item.ProductID
		quantities[index] = int64(item.Quantity)
	}

	rows, err := db.pool.Query(ctx, createOrderQuery, orderNumber, productIDs, quantities)
	if err != nil {
		return Order{}, mapCreateOrderError(err)
	}
	defer rows.Close()

	order, found, err := scanOrder(rows)
	if err != nil {
		return Order{}, mapCreateOrderError(err)
	}
	if !found {
		return Order{}, fmt.Errorf("create order: expected one order, got 0")
	}

	return order, nil
}

func (db *DB) GetOrder(ctx context.Context, orderID int64) (Order, error) {
	rows, err := db.pool.Query(ctx, orderQuery+` WHERE o.id = $1 ORDER BY oi.product_id`, orderID)
	if err != nil {
		return Order{}, fmt.Errorf("query order: %w", err)
	}
	defer rows.Close()

	order, found, err := scanOrder(rows)
	if err != nil {
		return Order{}, err
	}
	if !found {
		return Order{}, ErrOrderNotFound
	}
	return order, nil
}

const orderQuery = `
	SELECT o.id, o.order_number, o.created_at,
	       oi.product_id, p.name, oi.quantity
	FROM orders o
	JOIN order_items oi ON oi.order_id = o.id
	JOIN products p ON p.id = oi.product_id`

const createOrderQuery = `
	WITH new_order AS (
		INSERT INTO orders (order_number)
		SELECT $1
		WHERE cardinality($2::bigint[]) > 0
		RETURNING id, order_number, created_at
	), new_items AS (
		INSERT INTO order_items (order_id, product_id, quantity)
		SELECT new_order.id, input.product_id, input.quantity::integer
		FROM new_order
		CROSS JOIN unnest($2::bigint[], $3::bigint[]) AS input(product_id, quantity)
		RETURNING order_id, product_id, quantity
	)
	SELECT new_order.id, new_order.order_number, new_order.created_at,
	       new_items.product_id, products.name, new_items.quantity
	FROM new_order
	JOIN new_items ON new_items.order_id = new_order.id
	JOIN products ON products.id = new_items.product_id
	ORDER BY new_items.product_id`

func mapCreateOrderError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23503": // foreign_key_violation
			return ErrProductNotFound
		case pgErr.Code == "23505" && pgErr.ConstraintName == "order_items_pkey": // unique_violation
			return ErrDuplicateProduct
		}
	}
	return fmt.Errorf("create order: %w", err)
}

func scanOrder(rows pgx.Rows) (Order, bool, error) {
	var order Order
	found := false
	for rows.Next() {
		var rowOrder Order
		var item OrderItem
		if err := rows.Scan(
			&rowOrder.ID,
			&rowOrder.OrderNumber,
			&rowOrder.CreatedAt,
			&item.ProductID,
			&item.ProductName,
			&item.Quantity,
		); err != nil {
			return Order{}, false, fmt.Errorf("scan order: %w", err)
		}

		if !found {
			order = rowOrder
			order.Items = make([]OrderItem, 0, 1)
			found = true
		} else if rowOrder.ID != order.ID {
			return Order{}, false, fmt.Errorf("scan order: query returned multiple orders")
		}
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		return Order{}, false, fmt.Errorf("iterate order: %w", err)
	}

	return order, found, nil
}

func newOrderNumber() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate order number: %w", err)
	}

	return "ORD-" + strings.ToUpper(hex.EncodeToString(bytes)), nil
}
