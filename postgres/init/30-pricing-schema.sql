\connect db_pricing
SET ROLE user_pricing_service_team;

CREATE TABLE catalog (
    id SERIAL PRIMARY KEY,
    sku TEXT UNIQUE NOT NULL,
    unit_price NUMERIC(10, 2) NOT NULL
);

RESET ROLE;
