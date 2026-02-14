\connect db_pricing
SET ROLE user_pricing_service_team;

CREATE TABLE catalog (
    id SERIAL PRIMARY KEY,
    sku TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    brand TEXT,
    unit_price NUMERIC(10, 2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE catalog_new (
    id SERIAL PRIMARY KEY,
    sku TEXT UNIQUE NOT NULL,
    unit_price NUMERIC(10, 2) NOT NULL
);

INSERT INTO catalog_new (sku, unit_price)
SELECT sku, unit_price FROM catalog;

DROP TABLE catalog;
ALTER TABLE catalog_new RENAME TO catalog;

RESET ROLE;
