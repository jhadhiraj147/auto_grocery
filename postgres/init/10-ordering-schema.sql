\connect db_ordering
SET ROLE user_ordering_service_team;

CREATE TABLE smart_clients (
    id SERIAL PRIMARY KEY,
    device_id TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    phone TEXT,
    password_hash TEXT NOT NULL,
    card_info_enc TEXT,
    refresh_token TEXT,
    token_expiry TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE grocery_orders (
    id SERIAL PRIMARY KEY,
    order_id TEXT UNIQUE NOT NULL,
    client_id INT REFERENCES smart_clients(id),
    status TEXT NOT NULL DEFAULT 'PENDING',
    total_price NUMERIC(10, 2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE grocery_order_items (
    id SERIAL PRIMARY KEY,
    order_id INT REFERENCES grocery_orders(id) ON DELETE CASCADE,
    sku TEXT NOT NULL,
    quantity INT NOT NULL
);

CREATE INDEX idx_clients_device ON smart_clients(device_id);
CREATE INDEX idx_clients_token ON smart_clients(refresh_token);
CREATE INDEX idx_grocery_client ON grocery_orders(client_id);

CREATE TABLE suppliers (
    id SERIAL PRIMARY KEY,
    supplier_id TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE restock_orders (
    id SERIAL PRIMARY KEY,
    order_id TEXT UNIQUE NOT NULL,
    supplier_id INT REFERENCES suppliers(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'PENDING',
    total_cost NUMERIC(10,2) DEFAULT 0.00,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE restock_order_items (
    id SERIAL PRIMARY KEY,
    order_id INT REFERENCES restock_orders(id) ON DELETE CASCADE,
    sku TEXT NOT NULL,
    name TEXT NOT NULL,
    aisle_type TEXT,
    quantity INT NOT NULL,
    mfd_date TEXT,
    expiry_date TEXT,
    unit_cost NUMERIC(10,2) NOT NULL
);

CREATE INDEX idx_restock_supplier ON restock_orders(supplier_id);

RESET ROLE;
