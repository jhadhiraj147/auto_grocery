package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

type Item struct {
	ID        int     `json:"id"`
	Sku       string  `json:"sku"`
	UnitPrice float64 `json:"unit_price"`
}

type CatalogStore struct {
	db *sql.DB
}

// NewCatalogStore constructs a catalog store backed by postgres.
func NewCatalogStore(db *sql.DB) *CatalogStore {
	return &CatalogStore{db: db}
}

// UpsertItem inserts or updates a catalog sku and returns its row id.
func (s *CatalogStore) UpsertItem(ctx context.Context, item Item) (int, error) {
	log.Printf("[pricing-store] UpsertItem sku=%s unit_price=%.2f", item.Sku, item.UnitPrice)
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
		log.Printf("[pricing-store] UpsertItem failed sku=%s err=%v", item.Sku, err)
		return 0, fmt.Errorf("failed to upsert item %s: %w", item.Sku, err)
	}
	log.Printf("[pricing-store] UpsertItem success sku=%s id=%d", item.Sku, id)

	return id, nil
}

// GetItem fetches a catalog entry by sku.
func (s *CatalogStore) GetItem(ctx context.Context, sku string) (*Item, error) {
	log.Printf("[pricing-store] GetItem sku=%s", sku)
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
		log.Printf("[pricing-store] GetItem not found sku=%s", sku)
		return nil, fmt.Errorf("item not found")
	} else if err != nil {
		log.Printf("[pricing-store] GetItem failed sku=%s err=%v", sku, err)
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	log.Printf("[pricing-store] GetItem success sku=%s unit_price=%.2f", i.Sku, i.UnitPrice)
	return &i, nil
}

// GetItemsBySKUs fetches catalog entries for a sku batch lookup.
func (s *CatalogStore) GetItemsBySKUs(ctx context.Context, skus []string) (map[string]Item, error) {
	log.Printf("[pricing-store] GetItemsBySKUs count=%d skus=%v", len(skus), skus)
	query := `
        SELECT id, sku, unit_price
        FROM catalog
        WHERE sku = ANY($1)
    `
	rows, err := s.db.QueryContext(ctx, query, skus)
	if err != nil {
		log.Printf("[pricing-store] GetItemsBySKUs query failed err=%v", err)
		return nil, err
	}
	defer rows.Close()

	items := make(map[string]Item)
	for rows.Next() {
		var i Item
		err := rows.Scan(&i.ID, &i.Sku, &i.UnitPrice)
		if err != nil {
			log.Printf("[pricing-store] GetItemsBySKUs scan failed err=%v", err)
			return nil, err
		}
		items[i.Sku] = i
	}
	log.Printf("[pricing-store] GetItemsBySKUs success found=%d", len(items))
	return items, nil
}
