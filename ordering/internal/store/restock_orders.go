package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// 1. THE STRUCTS (Matching the DB Tables)
type RestockOrder struct {
	ID        int
	OrderID   string // Public ID (RES-999)
	TruckID   int    // Links back to SmartTruck.ID
	Status    string
	CreatedAt time.Time
}

type RestockOrderItem struct {
	ID       int
	OrderID  int // Links back to RestockOrder.ID
	Sku      string
	Quantity int
}

// We can reuse the same db connection, so we attach these to TruckStore
// or create a new struct. Let's create a dedicated store for clarity.
type RestockStore struct {
	db *sql.DB
}

func NewRestockStore(db *sql.DB) *RestockStore {
	return &RestockStore{db: db}
}

// ---------------------------------------------------------
// 2. REGISTER STOCK (Create Restock Order)
// ---------------------------------------------------------
// Usage: When a truck arrives and declares "I brought 50 Apples", we save it here.
func (s *RestockStore) CreateRestockOrder(ctx context.Context, order RestockOrder, items []RestockOrderItem) error {
	// A Transaction (Tx) ensures EITHER everything saves OR nothing saves.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // Safety switch

	// A. Save the Header (restock_orders)
	queryHeader := `
		INSERT INTO restock_orders (order_id, truck_id, status)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	// We get the new DB ID (e.g. Row #10) to link the items
	err = tx.QueryRowContext(ctx, queryHeader, order.OrderID, order.TruckID, "PENDING").Scan(&order.ID)
	if err != nil {
		return fmt.Errorf("failed to save restock header: %w", err)
	}

	// B. Save the Items (restock_order_items)
	queryItems := `
		INSERT INTO restock_order_items (order_id, sku, quantity)
		VALUES ($1, $2, $3)
	`
	stmt, err := tx.PrepareContext(ctx, queryItems)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		_, err := stmt.ExecContext(ctx, order.ID, item.Sku, item.Quantity)
		if err != nil {
			return fmt.Errorf("failed to save item %s: %w", item.Sku, err)
		}
	}

	// C. Commit (Save for real)
	return tx.Commit()
}

// ---------------------------------------------------------
// 3. GET HISTORY (All deliveries by this truck)
// ---------------------------------------------------------
func (s *RestockStore) GetRestockHistoryByTruckID(ctx context.Context, truckID int) ([]RestockOrder, error) {
	query := `
		SELECT id, order_id, status, created_at
		FROM restock_orders
		WHERE truck_id = $1
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, truckID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []RestockOrder
	for rows.Next() {
		var o RestockOrder
		if err := rows.Scan(&o.ID, &o.OrderID, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		o.TruckID = truckID
		history = append(history, o)
	}

	return history, nil
}

// ---------------------------------------------------------
// 4. GET LATEST STOCK (Last delivery by this truck)
// ---------------------------------------------------------
func (s *RestockStore) GetLastRestockByTruckID(ctx context.Context, truckID int) (*RestockOrder, error) {
	query := `
		SELECT id, order_id, status, created_at
		FROM restock_orders
		WHERE truck_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var o RestockOrder
	
	err := s.db.QueryRowContext(ctx, query, truckID).Scan(
		&o.ID, &o.OrderID, &o.Status, &o.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No history yet
	} else if err != nil {
		return nil, fmt.Errorf("failed to get last restock: %w", err)
	}

	o.TruckID = truckID
	return &o, nil
}