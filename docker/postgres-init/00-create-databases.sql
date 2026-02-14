CREATE USER user_ordering_service_team WITH PASSWORD 'ordering_secure_v1';
CREATE DATABASE db_ordering OWNER user_ordering_service_team;

CREATE USER user_inventory_service_team WITH PASSWORD 'inventory_secure_v1';
CREATE DATABASE db_inventory OWNER user_inventory_service_team;

CREATE USER user_pricing_service_team WITH PASSWORD 'pricing_secure_v1';
CREATE DATABASE db_pricing OWNER user_pricing_service_team;
