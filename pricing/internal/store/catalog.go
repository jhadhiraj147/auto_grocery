package store

import (
	"context"
	"database/sql"
	"fmt"
)

type Item struct {
	ID        int     `json:"id"`
	Sku       string  `json:"sku"`
	UnitPrice float64 `json:"unit_price"`
}

type CatalogStore struct {
	db *sql.DB
}

func NewCatalogStore(db *sql.DB) *CatalogStore {
	return &CatalogStore{db: db}
}

func (s *CatalogStore) UpsertItem(ctx context.Context, item Item) (int, error) {
	query := `
        INSERT INTO catalog (sku, unit_price)
        VALUES ($1, $2)
        ON CONFLICT (sku) 
        DO UPDATE SET 
            unit_price = EXCLUDED.unit_price
        RETURNING id
    `

	var id int
	err := s.db.QueryRowContext(ctx, query, item.Sku, item.UnitPrice).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to upsert item %s: %w", item.Sku, err)
	}

	return id, nil
}

func (s *CatalogStore) GetItem(ctx context.Context, sku string) (*Item, error) {
	query := `
        SELECT id, sku, unit_price
        FROM catalog
        WHERE sku = $1
    `
	var i Item
	err := s.db.QueryRowContext(ctx, query, sku).Scan(
		&i.ID, &i.Sku, &i.UnitPrice,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("item not found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	return &i, nil
}

func (s *CatalogStore) GetItemsBySKUs(ctx context.Context, skus []string) (map[string]Item, error) {
	query := `
        SELECT id, sku, unit_price
        FROM catalog
        WHERE sku = ANY($1)
    `
	rows, err := s.db.QueryContext(ctx, query, skus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make(map[string]Item)
	for rows.Next() {
		var i Item
		err := rows.Scan(&i.ID, &i.Sku, &i.UnitPrice)
		if err != nil {
			return nil, err
		}
		items[i.Sku] = i
	}
	return items, nil
}