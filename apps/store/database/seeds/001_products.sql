INSERT INTO products (name)
SELECT seed.name
FROM (
    VALUES
        ('Mechanical Keyboard'),
        ('Wireless Mouse'),
        ('USB-C Dock'),
        ('27-inch Monitor'),
        ('Noise-Cancelling Headphones')
) AS seed(name)
WHERE NOT EXISTS (
    SELECT 1
    FROM products
    WHERE products.name = seed.name
);
