package database

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNewOrderNumber(t *testing.T) {
	pattern := regexp.MustCompile(`^ORD-[0-9A-F]{16}$`)
	seen := make(map[string]struct{}, 1000)

	for range 1000 {
		orderNumber, err := newOrderNumber()
		if err != nil {
			t.Fatalf("generate order number: %v", err)
		}
		if !pattern.MatchString(orderNumber) {
			t.Fatalf("order number %q does not match expected format", orderNumber)
		}
		if _, exists := seen[orderNumber]; exists {
			t.Fatalf("generated duplicate order number %q", orderNumber)
		}
		seen[orderNumber] = struct{}{}
	}
}

func TestMapCreateOrderError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "missing product",
			err:  &pgconn.PgError{Code: "23503", ConstraintName: "order_items_product_id_fkey"},
			want: ErrProductNotFound,
		},
		{
			name: "duplicate product",
			err: fmt.Errorf("read rows: %w", &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "order_items_pkey",
			}),
			want: ErrDuplicateProduct,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mapCreateOrderError(test.err); !errors.Is(got, test.want) {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}

func TestMapCreateOrderErrorPreservesUnexpectedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "generic error", err: errors.New("database unavailable")},
		{
			name: "unrelated unique violation",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "orders_order_number_key",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mapCreateOrderError(test.err)
			if !errors.Is(got, test.err) {
				t.Fatalf("expected wrapped source error %v, got %v", test.err, got)
			}
			if errors.Is(got, ErrProductNotFound) || errors.Is(got, ErrDuplicateProduct) {
				t.Fatalf("unexpectedly mapped error: %v", got)
			}
		})
	}
}
