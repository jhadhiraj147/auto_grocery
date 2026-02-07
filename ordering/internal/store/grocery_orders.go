package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// 1. THE STRUCTS (Matching the DB Tables)
type GroceryOrder struct {
	ID         int
	OrderID    string // Public ID (ORD-101)
	ClientID   int
	Status     string
	TotalPrice float64
	CreatedAt  time.Time
}

type GroceryOrderItem struct {
	ID       int
	OrderID  int // Links back to GroceryOrder.ID
	Sku      string
	Quantity int
}

// Note: We use the same 'ClientStore' struct to hang these methods on,
// or we can make a new 'OrderStore'. Let's keep it simple and make a new one.
type OrderStore struct {
	db *sql.DB
}

func NewOrderStore(db *sql.DB) *OrderStore {
	return &OrderStore{db: db}
}

// ---------------------------------------------------------
// 2. CREATE ORDER (The "Shopping Cart" -> DB)
// ---------------------------------------------------------
func (s *OrderStore) CreateGroceryOrder(ctx context.Context, order GroceryOrder, items []GroceryOrderItem) error {
	// A Transaction (Tx) ensures EITHER everything saves OR nothing saves.
	// We don't want an Order Header without Items!
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // Safety switch: Undo everything if we panic

	// A. Save the Header (grocery_orders)
	queryHeader := `
		INSERT INTO grocery_orders (order_id, client_id, status, total_price)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	// We need the database ID (e.g. Row #5) to link the items
	err = tx.QueryRowContext(ctx, queryHeader, order.OrderID, order.ClientID, "PENDING", 0.0).Scan(&order.ID)
	if err != nil {
		return fmt.Errorf("failed to save order header: %w", err)
	}

	// B. Save the Items (grocery_order_items)
	queryItems := `
		INSERT INTO grocery_order_items (order_id, sku, quantity)
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
// 3. GET ORDER HISTORY (For a specific User)
// ---------------------------------------------------------
func (s *OrderStore) GetOrdersByClientID(ctx context.Context, clientID int) ([]GroceryOrder, error) {
	query := `
		SELECT id, order_id, status, total_price, created_at
		FROM grocery_orders
		WHERE client_id = $1
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []GroceryOrder
	for rows.Next() {
		var o GroceryOrder
		// Note: 'total_price' in DB is Numeric, here we scan to float64
		// If total_price is NULL, we might need sql.NullFloat64, but let's assume 0.00 for now.
		if err := rows.Scan(&o.ID, &o.OrderID, &o.Status, &o.TotalPrice, &o.CreatedAt); err != nil {
			return nil, err
		}
		o.ClientID = clientID
		history = append(history, o)
	}

	return history, nil
}

// ---------------------------------------------------------
// 4. GET LAST ORDER (Fast Lookup for "Receipt" screen)
// ---------------------------------------------------------
func (s *OrderStore) GetLastOrderByClientID(ctx context.Context, clientID int) (*GroceryOrder, error) {
	query := `
		SELECT id, order_id, status, total_price, created_at
		FROM grocery_orders
		WHERE client_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var o GroceryOrder
	
	// QueryRow is perfect here because we expect exactly one (or zero) results
	err := s.db.QueryRowContext(ctx, query, clientID).Scan(
		&o.ID, &o.OrderID, &o.Status, &o.TotalPrice, &o.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No error, just no history yet
	} else if err != nil {
		return nil, fmt.Errorf("failed to get last order: %w", err)
	}

	o.ClientID = clientID
	return &o, nil
}

// ---------------------------------------------------------
// 5. UPDATE ORDER STATUS (For Confirm/Cancel)
// ---------------------------------------------------------
func (s *OrderStore) UpdateOrderStatus(ctx context.Context, orderID string, status string, totalPrice float64) error {
	// If totalPrice is 0, we assume we just want to update status (like for Cancel)
	// If it's > 0, we update price too (like for Confirm)
	query := `
		UPDATE grocery_orders
		SET status = $1, total_price = CASE WHEN $2 > 0 THEN $2 ELSE total_price END
		WHERE order_id = $3
	`
	result, err := s.db.ExecContext(ctx, query, status, totalPrice, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("order not found: %s", orderID)
	}
	return nil
}


// ---------------------------------------------------------
// 6. GET ITEMS FOR AN ORDER (Needed for Cancel/Release)
// ---------------------------------------------------------
func (s *OrderStore) GetOrderItems(ctx context.Context, orderID string) ([]GroceryOrderItem, error) {
	// First get the integer ID of the order
	var dbID int
	err := s.db.QueryRowContext(ctx, "SELECT id FROM grocery_orders WHERE order_id = $1", orderID).Scan(&dbID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	query := `SELECT sku, quantity FROM grocery_order_items WHERE order_id = $1`
	rows, err := s.db.QueryContext(ctx, query, dbID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []GroceryOrderItem
	for rows.Next() {
		var i GroceryOrderItem
		if err := rows.Scan(&i.Sku, &i.Quantity); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

// ---------------------------------------------------------
// 7. DELETE ORDER (Hard Delete for Cancel)
// ---------------------------------------------------------
func (s *OrderStore) DeleteOrder(ctx context.Context, orderID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Get the DB ID
	var dbID int
	err = tx.QueryRowContext(ctx, "SELECT id FROM grocery_orders WHERE order_id = $1", orderID).Scan(&dbID)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	// 2. Delete Items first (Foreign Key constraint)
	_, err = tx.ExecContext(ctx, "DELETE FROM grocery_order_items WHERE order_id = $1", dbID)
	if err != nil {
		return fmt.Errorf("failed to delete items: %w", err)
	}

	// 3. Delete Header
	_, err = tx.ExecContext(ctx, "DELETE FROM grocery_orders WHERE id = $1", dbID)
	if err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}

	return tx.Commit()
}

// ---------------------------------------------------------
// 8. GET ORDER HEADER (For Security Checks)
// ---------------------------------------------------------
func (s *OrderStore) GetOrderByID(ctx context.Context, orderID string) (*GroceryOrder, error) {
	query := `
		SELECT id, order_id, client_id, status, total_price, created_at
		FROM grocery_orders
		WHERE order_id = $1
	`
	var o GroceryOrder
	err := s.db.QueryRowContext(ctx, query, orderID).Scan(
		&o.ID, &o.OrderID, &o.ClientID, &o.Status, &o.TotalPrice, &o.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &o, nil
}