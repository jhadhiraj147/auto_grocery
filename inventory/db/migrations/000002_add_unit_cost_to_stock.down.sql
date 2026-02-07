-- Revert the change: Remove the unit_cost column
ALTER TABLE available_stock 
DROP COLUMN unit_cost;