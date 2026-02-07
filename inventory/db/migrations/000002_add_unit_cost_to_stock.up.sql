-- Apply the change: Add the unit_cost column
ALTER TABLE available_stock 
ADD COLUMN unit_cost NUMERIC(10, 2) DEFAULT 0.00;