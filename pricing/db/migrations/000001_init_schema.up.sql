CREATE TABLE catalog (
    id SERIAL PRIMARY KEY,         -- The computer's internal ID (1, 2, 3...)
    sku TEXT UNIQUE NOT NULL,      -- The Barcode (e.g. "SN-APPL-001")
    name TEXT NOT NULL,            -- The Name (e.g. "Gala Apple")
    brand TEXT,                    -- The Brand (e.g. "Organic Farms")
    unit_price NUMERIC(10, 2) NOT NULL, -- The Price (e.g. 0.99)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP -- When did we add this?
);