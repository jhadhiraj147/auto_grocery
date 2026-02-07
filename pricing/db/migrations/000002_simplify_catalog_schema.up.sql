-- 1. Create the new consistent table
CREATE TABLE catalog_new (
    id SERIAL PRIMARY KEY,
    sku TEXT UNIQUE NOT NULL,
    unit_price NUMERIC(10, 2) NOT NULL
);

-- 2. Migrate existing data (sku and price)
INSERT INTO catalog_new (sku, unit_price)
SELECT sku, unit_price FROM catalog;

-- 3. Swap the tables
DROP TABLE catalog;
ALTER TABLE catalog_new RENAME TO catalog;