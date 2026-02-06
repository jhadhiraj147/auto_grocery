CREATE TABLE available_stock (
    id SERIAL PRIMARY KEY,         -- Uniform ID (1, 2, 3...) just like Pricing
    sku TEXT UNIQUE NOT NULL,      -- The Business Key (Must be Unique)
    name TEXT NOT NULL,            -- Human readable name
    
    aisle_type TEXT NOT NULL,      -- The Filter Tag (Meat, Produce, etc.)
    quantity INT NOT NULL DEFAULT 0, 
    
    mfd_date TIMESTAMP,            -- Manufacturing Date
    expiry_date TIMESTAMP,         -- Expiration Date
    
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast filtering by aisle
CREATE INDEX idx_aisle_type ON available_stock(aisle_type);