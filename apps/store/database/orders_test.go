package database

import (
	"regexp"
	"testing"
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
