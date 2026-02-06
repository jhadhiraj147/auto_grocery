package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq" // Ensure you have this imported for pq.Array
)

// 1. THE DATA STRUCTURE
// Represents a row in the 'available_stock' table.
type StockItem struct {
	ID          int
	Sku         string
	Name        string
	AisleType   string
	Quantity    int32
	MfdDate     time.Time
	ExpiryDate  time.Time
	LastUpdated time.Time
}

// 2. THE STORE OBJECT
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// 3. GET BATCH (Read Only - For UI)
// Usage: UI wants to see if items exist.
// Output: Full details (Name, Price, etc.) because the UI needs them.
func (s *Store) GetBatchItems(ctx context.Context, skus []string) (map[string]*StockItem, error) {
	query := `
        SELECT id, sku, name, aisle_type, quantity, mfd_date, expiry_date, last_updated
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
			&i.ID, &i.Sku, &i.Name, &i.AisleType, &i.Quantity,
			&i.MfdDate, &i.ExpiryDate, &i.LastUpdated,
		); err != nil {
			return nil, err
		}
		items[i.Sku] = &i
	}
	return items, nil
}

// 4. RESERVE STOCK (The "Checkout" - Updated for Symmetry)
// Logic: Locks rows, calculates partial fill (LEAST), and subtracts stock.
// Input:  {"apple": 5, "banana": 10}
// Output: {"apple": 5, "banana": 3}  <-- Simple Map of what was actually taken
func (s *Store) ReserveStock(ctx context.Context, requests map[string]int32) (map[string]int32, error) {
	// Prepare Arrays for Bulk Query
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
            WHERE s.quantity > 0 -- Optimization: Ignore if already empty
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

	// Fill the simple map result
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

// 5. RELEASE STOCK (The "Undo" Button)
// Usage: If User cancels, or Payment fails, call this with the map you got from ReserveStock.
// Input: {"apple": 5, "banana": 3} -> Adds exactly this amount back.
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

// 6. UPSERT STOCK (The "Truck" Delivery)
// Usage: Only used when new stock arrives (requires Name, Dates, Aisle).
func (s *Store) UpsertStock(ctx context.Context, item StockItem) error {
	query := `
        INSERT INTO available_stock (sku, name, aisle_type, quantity, mfd_date, expiry_date, last_updated)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
        ON CONFLICT (sku) 
        DO UPDATE SET 
            quantity = available_stock.quantity + EXCLUDED.quantity, -- Add to existing
            name = EXCLUDED.name,
            last_updated = NOW()
    `
	_, err := s.db.ExecContext(ctx, query,
		item.Sku, item.Name, item.AisleType, item.Quantity, item.MfdDate, item.ExpiryDate,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert stock: %w", err)
	}
	return nil
}

// 7. CLEAR EXPIRED STOCK (Housekeeping)
// Usage: Cron job to remove spoiled items.
func (s *Store) ClearExpiredStock(ctx context.Context) (int64, error) {
	query := `
        UPDATE available_stock
        SET quantity = 0, 
            expiry_date = '0001-01-01 00:00:00', 
            last_updated = NOW()
        WHERE expiry_date < NOW() AND quantity > 0
    `

	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to clear expired stock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}