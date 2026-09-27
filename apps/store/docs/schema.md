# Schema

## Products

- `id` - Primary key
- `name` - Required
- `created_at` - Timestamp with timezone, defaults to the current time

## Orders

- `id` - Primary key
- `order_number` - Required and globally unique
- `created_at` - Timestamp with timezone, defaults to the current time

## Order Items

- `order_id` - Foreign key referencing `Orders.id`
- `product_id` - Foreign key referencing `Products.id`
- `quantity` - Required positive integer

The composite primary key is (`order_id`, `product_id`). It prevents duplicate
product rows within an order; `quantity` records how many units of the product
were ordered.

## Relationships

- One order has many order items.
- One product can belong to many order items.
- One order can therefore contain multiple products.

## Indexes

- Composite primary key on `Order Items (order_id, product_id)`.
- Index on `Order Items.product_id` for product-based lookups.
- B-tree index on `Products.name` using `text_pattern_ops` for prefix searches.
