package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type StockItem struct {
	ID         int64     `json:"id"`
	SKU        string    `json:"sku"`
	Name       string    `json:"name"`
	AisleType  string    `json:"aisle_type"`
	Quantity   int       `json:"quantity"`
	UnitCost   float64   `json:"unit_cost"`
	MfdDate    time.Time `json:"mfd_date"`
	ExpiryDate time.Time `json:"expiry_date"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetBatchItems(ctx context.Context, skus []string) (map[string]*StockItem, error) {

	query := `
        SELECT id, sku, name, aisle_type, quantity, unit_cost, mfd_date, expiry_date, last_updated
        FROM available_stock
        WHERE sku = ANY($1)
    `
	rows, err := s.db.QueryContext(ctx, query, pq.Array(skus))
	if err != nil {
		return nil, fmt.Errorf("failed to batch get items: %w", err)
	}
	defer rows.Close()

	items := make(map[string]*StockItem)
	for rows.Next() {
		var i StockItem
		if err := rows.Scan(
			&i.ID, &i.SKU, &i.Name, &i.AisleType, &i.Quantity, &i.UnitCost,
			&i.MfdDate, &i.ExpiryDate, &i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items[i.SKU] = &i
	}
	return items, nil
}

func (s *Store) ReserveStock(ctx context.Context, requests map[string]int32) (map[string]int32, error) {
	var skus []string
	var counts []int32
	for sku, count := range requests {
		skus = append(skus, sku)
		counts = append(counts, count)
	}

	query := `
        WITH request_batch AS (
            SELECT unnest($1::text[]) as sku, unnest($2::int[]) as count
        ),
        locked_rows AS (
            SELECT s.id, s.sku, s.quantity as old_qty, r.count as req_count
            FROM available_stock s
            JOIN request_batch r ON s.sku = r.sku
            WHERE s.quantity > 0 
            FOR UPDATE
        )
        UPDATE available_stock s
        SET quantity = s.quantity - LEAST(l.old_qty, l.req_count), 
            last_updated = NOW()
        FROM locked_rows l
        WHERE s.id = l.id
        RETURNING s.sku, LEAST(l.old_qty, l.req_count) as actual_taken
    `

	rows, err := s.db.QueryContext(ctx, query, pq.Array(skus), pq.Array(counts))
	if err != nil {
		return nil, fmt.Errorf("failed to reserve stock: %w", err)
	}
	defer rows.Close()

	results := make(map[string]int32)
	for rows.Next() {
		var sku string
		var taken int32
		if err := rows.Scan(&sku, &taken); err != nil {
			return nil, err
		}
		results[sku] = taken
	}
	return results, nil
}

func (s *Store) ReleaseStock(ctx context.Context, returns map[string]int32) error {
	var skus []string
	var counts []int32
	for sku, count := range returns {
		skus = append(skus, sku)
		counts = append(counts, count)
	}

	query := `
        UPDATE available_stock s
        SET quantity = s.quantity + data.count,
            last_updated = NOW()
        FROM (
            SELECT unnest($1::text[]) as sku, unnest($2::int[]) as count
        ) as data
        WHERE s.sku = data.sku
    `
	_, err := s.db.ExecContext(ctx, query, pq.Array(skus), pq.Array(counts))
	if err != nil {
		return fmt.Errorf("failed to release stock: %w", err)
	}
	return nil
}

func (s *Store) UpsertStock(ctx context.Context, item StockItem) error {
	query := `
        INSERT INTO available_stock (sku, name, aisle_type, quantity, unit_cost, mfd_date, expiry_date, last_updated)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
        ON CONFLICT (sku) 
        DO UPDATE SET 
            quantity = available_stock.quantity + EXCLUDED.quantity, 
			unit_cost = EXCLUDED.unit_cost,
            name = EXCLUDED.name,
            last_updated = NOW()
    `
	_, err := s.db.ExecContext(ctx, query,
		item.SKU,
		item.Name,
		item.AisleType,
		item.Quantity,
		item.UnitCost,
		item.MfdDate,
		item.ExpiryDate,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert stock: %w", err)
	}
	return nil
}

func (s *Store) ClearExpiredStock(ctx context.Context) (int64, error) {
	query := `
        UPDATE available_stock
        SET quantity = 0, 
            last_updated = NOW()
        WHERE expiry_date < NOW() AND quantity > 0
    `
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to clear expired stock: %w", err)
	}
	return result.RowsAffected()
}

func (s *Store) GetAllStock(ctx context.Context) ([]StockItem, error) {
	query := `SELECT sku, quantity, unit_cost FROM available_stock`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all stock: %w", err)
	}
	defer rows.Close()

	var items []StockItem
	for rows.Next() {
		var i StockItem
		if err := rows.Scan(&i.SKU, &i.Quantity, &i.UnitCost); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}
