-- Change column from INT to NUMERIC to support decimals
ALTER TABLE grocery_orders 
ALTER COLUMN total_price TYPE NUMERIC(10,2);