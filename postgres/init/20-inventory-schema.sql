\connect db_inventory
SET ROLE user_inventory_service_team;

CREATE TABLE available_stock (
    id SERIAL PRIMARY KEY,
    sku TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    aisle_type TEXT NOT NULL,
    quantity INT NOT NULL DEFAULT 0,
    mfd_date TIMESTAMP,
    expiry_date TIMESTAMP,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_aisle_type ON available_stock(aisle_type);

ALTER TABLE available_stock
ADD COLUMN unit_cost NUMERIC(10, 2) DEFAULT 0.00;

RESET ROLE;
