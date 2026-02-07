-- =========================================================
-- 1. SMART CLIENTS (IoT Fridges / Users)
-- =========================================================
CREATE TABLE smart_clients (
    id SERIAL PRIMARY KEY,
    
    -- Identity
    device_id TEXT UNIQUE NOT NULL,    -- The "Username" (e.g. "FRIDGE-99-X")
    email TEXT UNIQUE NOT NULL,        -- For notifications
    phone TEXT,                        -- For SMS updates
    
    -- Security
    password_hash TEXT NOT NULL,       -- Hashed Password
    
    -- Payment (Encrypted String)
    -- We assume the encryption happens in Go before saving here
    card_info_enc TEXT,                
    
    -- Session Management (JWT)
    -- We store the token to allow "Revoke Access" (Force Logout)
    refresh_token TEXT,                
    token_expiry TIMESTAMP,            -- When does this session die?
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =========================================================
-- 2. GROCERY ORDERS (The Transaction Header)
-- =========================================================
CREATE TABLE grocery_orders (
    id SERIAL PRIMARY KEY,
    
    -- Public ID passed to Inventory (e.g. "ORD-101")
    order_id TEXT UNIQUE NOT NULL,     
    
    -- LINK: Connects this order to the specific Smart Fridge
    client_id INT REFERENCES smart_clients(id), 
    
    status TEXT NOT NULL DEFAULT 'PENDING',
    total_price NUMERIC(10, 2),        -- Filled by Pricing Service
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =========================================================
-- 3. GROCERY ITEMS (The Shopping List)
-- =========================================================
CREATE TABLE grocery_order_items (
    id SERIAL PRIMARY KEY,
    
    -- LINK: Connects these items to the specific Order above
    order_id INT REFERENCES grocery_orders(id) ON DELETE CASCADE,
    
    sku TEXT NOT NULL,
    quantity INT NOT NULL
);

-- Indexes for fast lookup
CREATE INDEX idx_clients_device ON smart_clients(device_id);
CREATE INDEX idx_clients_token ON smart_clients(refresh_token); -- Fast Auth checks
CREATE INDEX idx_grocery_client ON grocery_orders(client_id);   -- Fast Order History