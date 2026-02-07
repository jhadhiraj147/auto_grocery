-- =========================================================
-- 1. SMART TRUCKS (Suppliers)
-- =========================================================
CREATE TABLE smart_trucks (
    id SERIAL PRIMARY KEY,
    
    -- Identity
    truck_id TEXT UNIQUE NOT NULL,     -- e.g. "TRUCK-55-B"
    plate_number TEXT,                 -- License Plate
    driver_name TEXT,                  -- Who is driving?
    
    -- Contact & Location (Added as requested)
    contact_info TEXT,                 -- Phone number or Radio ID
    location TEXT,                     -- e.g. "Warehouse A", "En Route", or GPS coords
    
    -- No password needed for V1 (Trusted System)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =========================================================
-- 2. RESTOCK ORDERS (The Shipment Header)
-- =========================================================
CREATE TABLE restock_orders (
    id SERIAL PRIMARY KEY,
    
    -- Public ID passed to Inventory (e.g. "RES-999")
    order_id TEXT UNIQUE NOT NULL,     
    
    -- LINK: Connects this shipment to the specific Truck
    truck_id INT REFERENCES smart_trucks(id), 
    
    status TEXT NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =========================================================
-- 3. RESTOCK ITEMS (The Cargo List)
-- =========================================================
CREATE TABLE restock_order_items (
    id SERIAL PRIMARY KEY,
    
    -- LINK: Connects these items to the specific Restock Order
    order_id INT REFERENCES restock_orders(id) ON DELETE CASCADE,
    
    sku TEXT NOT NULL,
    quantity INT NOT NULL
);

-- Indexes for fast lookup
CREATE INDEX idx_trucks_id ON smart_trucks(truck_id);
CREATE INDEX idx_restock_truck ON restock_orders(truck_id);