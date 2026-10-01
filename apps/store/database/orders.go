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

	orders, err := scanOrders(rows)
	if err != nil {
		return Order{}, mapCreateOrderError(err)
	}
	if len(orders) != 1 {
		return Order{}, fmt.Errorf("create order: expected one order, got %d", len(orders))
	}

	return orders[0], nil
}

func (db *DB) GetOrder(ctx context.Context, orderID int64) (Order, error) {
	rows, err := db.pool.Query(ctx, orderQuery+` WHERE o.id = $1 ORDER BY oi.product_id`, orderID)
	if err != nil {
		return Order{}, fmt.Errorf("query order: %w", err)
	}
	defer rows.Close()

	orders, err := scanOrders(rows)
	if err != nil {
		return Order{}, err
	}
	if len(orders) == 0 {
		return Order{}, ErrOrderNotFound
	}
	if len(orders) != 1 {
		return Order{}, fmt.Errorf("query order: expected one order, got %d", len(orders))
	}

	return orders[0], nil
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

func scanOrders(rows pgx.Rows) ([]Order, error) {
	orders := make([]Order, 0)
	orderIndexes := make(map[int64]int)

	for rows.Next() {
		var order Order
		var item OrderItem
		if err := rows.Scan(
			&order.ID,
			&order.OrderNumber,
			&order.CreatedAt,
			&item.ProductID,
			&item.ProductName,
			&item.Quantity,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}

		index, exists := orderIndexes[order.ID]
		if !exists {
			order.Items = make([]OrderItem, 0, 1)
			orders = append(orders, order)
			index = len(orders) - 1
			orderIndexes[order.ID] = index
		}
		orders[index].Items = append(orders[index].Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}

	return orders, nil
}

func newOrderNumber() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate order number: %w", err)
	}

	return "ORD-" + strings.ToUpper(hex.EncodeToString(bytes)), nil
}
