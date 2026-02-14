package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type RestockStore struct {
	db *sql.DB
}

// NewRestockStore constructs a restock store backed by postgres.
func NewRestockStore(db *sql.DB) *RestockStore {
	return &RestockStore{db: db}
}

type RestockOrder struct {
	ID         int
	OrderID    string
	SupplierID int // Internal DB ID from suppliers table
	Status     string
	TotalCost  float64
	CreatedAt  time.Time
}

type RestockOrderItem struct {
	ID         int
	OrderID    int // Internal DB ID
	Sku        string
	Name       string
	AisleType  string
	Quantity   int
	MfdDate    string
	ExpiryDate string
	UnitCost   float64
}

// GetSupplierInternalID upserts supplier metadata and returns internal numeric id.
func (s *RestockStore) GetSupplierInternalID(ctx context.Context, businessID string, supplierName string) (int, error) {
	var id int
	query := `
        INSERT INTO suppliers (supplier_id, name)
        VALUES ($1, $2)
        ON CONFLICT (supplier_id) DO UPDATE SET name = EXCLUDED.name
        RETURNING id
    `
	err := s.db.QueryRowContext(ctx, query, businessID, supplierName).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get or create supplier %s: %w", businessID, err)
	}
	return id, nil
}

// CreateRestockOrder writes restock order header and items atomically.
func (s *RestockStore) CreateRestockOrder(ctx context.Context, order *RestockOrder, items []RestockOrderItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Insert Order Header
	// We use RETURNING created_at to populate the struct immediately
	queryHeader := `
        INSERT INTO restock_orders (order_id, supplier_id, status, total_cost)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at
    `
	err = tx.QueryRowContext(ctx, queryHeader, order.OrderID, order.SupplierID, order.Status, order.TotalCost).Scan(&order.ID, &order.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to save restock order header: %w", err)
	}

	// 2. Insert Items
	queryItems := `
        INSERT INTO restock_order_items 
        (order_id, sku, name, aisle_type, quantity, mfd_date, expiry_date, unit_cost) 
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `
	stmt, err := tx.PrepareContext(ctx, queryItems)
	if err != nil {
		return fmt.Errorf("failed to prepare restock items statement: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		_, err := stmt.ExecContext(ctx,
			order.ID,
			item.Sku,
			item.Name,
			item.AisleType,
			item.Quantity,
			item.MfdDate,
			item.ExpiryDate,
			item.UnitCost,
		)
		if err != nil {
			return fmt.Errorf("failed to save restock item %s: %w", item.Sku, err)
		}
	}

	return tx.Commit()
}

// UpdateOrderStatus updates restock order status and finalized total cost.
func (s *RestockStore) UpdateOrderStatus(ctx context.Context, businessOrderID string, status string, totalCost float64) error {
	query := `
        UPDATE restock_orders
        SET status = $1, total_cost = $2
        WHERE order_id = $3
    `
	res, err := s.db.ExecContext(ctx, query, status, totalCost, businessOrderID)
	if err != nil {
		return fmt.Errorf("failed to update restock order %s: %w", businessOrderID, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no restock order found with order_id %s", businessOrderID)
	}

	return nil
}

// GetRestockOrder fetches a restock order by business order id.
func (s *RestockStore) GetRestockOrder(ctx context.Context, orderID string) (*RestockOrder, error) {
	query := `
		SELECT id, order_id, supplier_id, status, total_cost, created_at
		FROM restock_orders
		WHERE order_id = $1
	`
	var o RestockOrder
	err := s.db.QueryRowContext(ctx, query, orderID).Scan(
		&o.ID, &o.OrderID, &o.SupplierID, &o.Status, &o.TotalCost, &o.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("restock order not found: %w", err)
	}

	o.CreatedAt = time.Date(
		o.CreatedAt.Year(), o.CreatedAt.Month(), o.CreatedAt.Day(),
		o.CreatedAt.Hour(), o.CreatedAt.Minute(), o.CreatedAt.Second(),
		o.CreatedAt.Nanosecond(), time.Local,
	)

	return &o, nil
}
