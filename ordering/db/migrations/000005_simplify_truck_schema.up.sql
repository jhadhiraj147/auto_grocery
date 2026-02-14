-- 1. Wipe the old, bloated tables
DROP TABLE IF EXISTS restock_order_items;
DROP TABLE IF EXISTS restock_orders;
DROP TABLE IF EXISTS smart_trucks;
DROP TABLE IF EXISTS suppliers;


-- 2. Suppliers (Exactly matches smart_clients)
CREATE TABLE suppliers (
    id SERIAL PRIMARY KEY,              
    supplier_id TEXT UNIQUE NOT NULL,   -- Business ID (e.g., "SUPP-NESTLE-01")
    name TEXT NOT NULL,                 -- NEW: Supplier Name (e.g., "Nestle Waters")
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 3. Restock Orders (Exactly matches grocery_orders)
CREATE TABLE restock_orders (
    id SERIAL PRIMARY KEY,              -- Internal DB ID
    order_id TEXT UNIQUE NOT NULL,      -- Business ID (e.g., "RESTOCK_123")
    supplier_id INT REFERENCES suppliers(id) ON DELETE CASCADE, -- Foreign Key to INT
    status TEXT NOT NULL DEFAULT 'PENDING',
    total_cost NUMERIC(10,2) DEFAULT 0.00,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 4. Restock Items (Exactly matches grocery_order_items)
CREATE TABLE restock_order_items (
    id SERIAL PRIMARY KEY,
    order_id INT REFERENCES restock_orders(id) ON DELETE CASCADE, -- Foreign Key to INT
    sku TEXT NOT NULL,
    name TEXT NOT NULL,
    aisle_type TEXT,
    quantity INT NOT NULL,
    mfd_date TEXT,
    expiry_date TEXT,
    unit_cost NUMERIC(10,2) NOT NULL
);