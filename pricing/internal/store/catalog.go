package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// 1. THE DATA STRUCTURE
// This mirrors the 'catalog' table you just created in the DB.
type Item struct {
	ID        int
	Sku       string
	Name      string
	Brand     string
	UnitPrice float64 // Maps to NUMERIC in DB
	CreatedAt time.Time
}

// 2. THE STORE OBJECT
// This holds the connection pool.
type CatalogStore struct {
	db *sql.DB
}

// NewCatalogStore creates a new Store.
// It expects 'main.go' to pass it a working database connection.
func NewCatalogStore(db *sql.DB) *CatalogStore {
	return &CatalogStore{db: db}
}

// 3. THE "CREATE" METHOD (Write to DB)
func (s *CatalogStore) CreateItem(ctx context.Context, item Item) (int, error) {
	query := `
		INSERT INTO catalog (sku, name, brand, unit_price)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	var id int
	err := s.db.QueryRowContext(ctx, query, item.Sku, item.Name, item.Brand, item.UnitPrice).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert item: %w", err)
	}
	return id, nil
}

// 4. THE "GET" METHOD (Read from DB)
func (s *CatalogStore) GetItem(ctx context.Context, sku string) (*Item, error) {
	query := `
		SELECT id, sku, name, brand, unit_price, created_at
		FROM catalog
		WHERE sku = $1
	`
	var i Item
	err := s.db.QueryRowContext(ctx, query, sku).Scan(
		&i.ID, &i.Sku, &i.Name, &i.Brand, &i.UnitPrice, &i.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("item not found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	return &i, nil

}

// 6. THE "UPSERT" METHOD (Smart Insert/Update)
// If item exists -> Update Price. If new -> Create it.
func (s *CatalogStore) UpsertItem(ctx context.Context, item Item) (int, error) {
	query := `
		INSERT INTO catalog (sku, name, brand, unit_price)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (sku) 
		DO UPDATE SET 
			unit_price = EXCLUDED.unit_price,
			name = EXCLUDED.name,
			brand = EXCLUDED.brand
		RETURNING id
	`

	var id int
	// We use the same scan logic because RETURNING id works for both insert and update
	err := s.db.QueryRowContext(ctx, query, item.Sku, item.Name, item.Brand, item.UnitPrice).Scan(&id)
	
	if err != nil {
		return 0, fmt.Errorf("failed to upsert item: %w", err)
	}

	return id, nil
}
