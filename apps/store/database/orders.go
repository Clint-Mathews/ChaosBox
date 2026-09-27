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
)

func (db *DB) CreateOrder(ctx context.Context, items []NewOrderItem) (Order, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return Order{}, fmt.Errorf("begin order transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	orderNumber, err := newOrderNumber()
	if err != nil {
		return Order{}, err
	}

	var orderID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO orders (order_number) VALUES ($1) RETURNING id`,
		orderNumber,
	).Scan(&orderID); err != nil {
		return Order{}, fmt.Errorf("insert order: %w", err)
	}

	for _, item := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO order_items (order_id, product_id, quantity)
			 VALUES ($1, $2, $3)`,
			orderID,
			item.ProductID,
			item.Quantity,
		); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				switch pgErr.Code {
				case "23503": // foreign_key_violation
					return Order{}, ErrProductNotFound
				case "23505": // unique_violation
					return Order{}, ErrDuplicateProduct
				}
			}
			return Order{}, fmt.Errorf("insert order item: %w", err)
		}
	}

	order, err := queryOrder(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("commit order transaction: %w", err)
	}

	return order, nil
}

func (db *DB) ListOrders(ctx context.Context) ([]Order, error) {
	rows, err := db.pool.Query(ctx, orderQuery+` ORDER BY o.id, oi.product_id`)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	return scanOrders(rows)
}

const orderQuery = `
	SELECT o.id, o.order_number, o.created_at,
	       oi.product_id, p.name, oi.quantity
	FROM orders o
	JOIN order_items oi ON oi.order_id = o.id
	JOIN products p ON p.id = oi.product_id`

func queryOrder(ctx context.Context, tx pgx.Tx, orderID int64) (Order, error) {
	rows, err := tx.Query(ctx, orderQuery+` WHERE o.id = $1 ORDER BY oi.product_id`, orderID)
	if err != nil {
		return Order{}, fmt.Errorf("query created order: %w", err)
	}
	defer rows.Close()

	orders, err := scanOrders(rows)
	if err != nil {
		return Order{}, err
	}
	if len(orders) != 1 {
		return Order{}, fmt.Errorf("query created order: expected one order, got %d", len(orders))
	}

	return orders[0], nil
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
