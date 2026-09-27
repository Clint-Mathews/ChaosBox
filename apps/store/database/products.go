package database

import (
	"context"
	"fmt"
)

func (db *DB) ListProducts(ctx context.Context, namePrefix string) ([]Product, error) {
	query := `SELECT id, name, created_at FROM products`
	args := []any{}
	if namePrefix != "" {
		query += ` WHERE name LIKE $1`
		args = append(args, namePrefix+"%")
	}
	query += ` ORDER BY id`

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		var product Product
		if err := rows.Scan(&product.ID, &product.Name, &product.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}

	return products, nil
}
