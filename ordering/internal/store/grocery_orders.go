package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type GroceryOrder struct {
	ID         int       `json:"-"`
	OrderID    string    `json:"OrderID"`
	ClientID   int       `json:"-"`
	DeviceID   string    `json:"DeviceID"`
	Status     string    `json:"Status"`
	TotalPrice float64   `json:"TotalPrice"`
	CreatedAt  time.Time `json:"CreatedAt"`
}

type GroceryOrderItem struct {
	ID       int
	OrderID  int
	Sku      string
	Quantity int
}

type OrderStore struct {
	db *sql.DB
}

// NewOrderStore constructs an order store backed by postgres.
func NewOrderStore(db *sql.DB) *OrderStore {
	return &OrderStore{db: db}
}

// CreateGroceryOrder writes an order header and item rows in a single transaction.
func (s *OrderStore) CreateGroceryOrder(ctx context.Context, order GroceryOrder, items []GroceryOrderItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Insert the Order Header
	queryHeader := `
		INSERT INTO grocery_orders (order_id, client_id, status, total_price)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	// Note: We scan the generated DB ID into 'order.ID' for use in the items loop below
	err = tx.QueryRowContext(ctx, queryHeader, order.OrderID, order.ClientID, "PENDING", 0.0).Scan(&order.ID)
	if err != nil {
		return fmt.Errorf("failed to save order header: %w", err)
	}

	// 2. Insert the Items
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
		// Use the integer ID from the created order
		_, err := stmt.ExecContext(ctx, order.ID, item.Sku, item.Quantity)
		if err != nil {
			return fmt.Errorf("failed to save item %s: %w", item.Sku, err)
		}
	}

	return tx.Commit()
}

// GetPendingOrderForClient returns the first PENDING order owned by clientID, or nil if none exists.
func (s *OrderStore) GetPendingOrderForClient(ctx context.Context, clientID int) (*GroceryOrder, error) {
	var o GroceryOrder
	err := s.db.QueryRowContext(ctx,
		`SELECT id, order_id, client_id, status FROM grocery_orders WHERE client_id = $1 AND status = 'PENDING' LIMIT 1`,
		clientID,
	).Scan(&o.ID, &o.OrderID, &o.ClientID, &o.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check pending order: %w", err)
	}
	return &o, nil
}

// GetOrdersByClientID returns the most recent orders for a given client, capped by limit.
func (s *OrderStore) GetOrdersByClientID(ctx context.Context, clientID int, limit int) ([]GroceryOrder, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `
		SELECT g.id, g.order_id, sc.device_id, g.status, g.total_price, g.created_at
		FROM grocery_orders g
		JOIN smart_clients sc ON sc.id = g.client_id
		WHERE g.client_id = $1
		ORDER BY g.created_at DESC
		LIMIT $2
	`
	rows, err := s.db.QueryContext(ctx, query, clientID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []GroceryOrder
	for rows.Next() {
		var o GroceryOrder
		if err := rows.Scan(&o.ID, &o.OrderID, &o.DeviceID, &o.Status, &o.TotalPrice, &o.CreatedAt); err != nil {
			return nil, err
		}
		o.ClientID = clientID
		history = append(history, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return history, nil
}

// GetLastOrderByClientID returns the latest order for polling UX.
func (s *OrderStore) GetLastOrderByClientID(ctx context.Context, clientID int) (*GroceryOrder, error) {
	query := `
		SELECT g.id, g.order_id, sc.device_id, g.status, g.total_price, g.created_at
		FROM grocery_orders g
		JOIN smart_clients sc ON sc.id = g.client_id
		WHERE g.client_id = $1
		ORDER BY g.created_at DESC
		LIMIT 1
	`
	var o GroceryOrder

	err := s.db.QueryRowContext(ctx, query, clientID).Scan(
		&o.ID, &o.OrderID, &o.DeviceID, &o.Status, &o.TotalPrice, &o.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get last order: %w", err)
	}

	o.ClientID = clientID
	return &o, nil
}

// UpdateOrderStatus updates business status and finalized total price.
// Only transitions from PROCESSING state — prevents webhook from bypassing the state machine
// (e.g. PENDING→COMPLETED or COMPLETED→FAILED which would silently corrupt order history).
func (s *OrderStore) UpdateOrderStatus(ctx context.Context, orderID string, status string, totalPrice float64) error {
	query := `
		UPDATE grocery_orders
		SET status = $1,
		    total_price = CASE WHEN $2::NUMERIC > 0 THEN $2::NUMERIC ELSE total_price END
		WHERE order_id = $3 AND status = 'PROCESSING'
	`
	result, err := s.db.ExecContext(ctx, query, status, totalPrice, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("order not found or not in PROCESSING state: %s", orderID)
	}
	return nil
}

// GetOrderItems returns item lines for a business order id.
func (s *OrderStore) GetOrderItems(ctx context.Context, orderID string) ([]GroceryOrderItem, error) {
	// Single JOIN query eliminates the extra round-trip for the internal integer ID.
	query := `
		SELECT i.sku, i.quantity
		FROM grocery_order_items i
		JOIN grocery_orders o ON o.id = i.order_id
		WHERE o.order_id = $1
	`
	rows, err := s.db.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return items, nil
}

// DeleteOrder removes an order header; items are removed automatically via ON DELETE CASCADE.
func (s *OrderStore) DeleteOrder(ctx context.Context, orderID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM grocery_orders WHERE order_id = $1", orderID)
	if err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("order not found: %s", orderID)
	}
	return nil
}

// GetOrderByID fetches a single order by business order id.
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

// UpdateStatus updates only the lifecycle status for an order.
func (s *OrderStore) UpdateStatus(ctx context.Context, orderID string, status string) error {
	query := `UPDATE grocery_orders SET status = $1 WHERE order_id = $2`
	result, err := s.db.ExecContext(ctx, query, status, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("order not found: %s", orderID)
	}
	return nil
}

// TransitionOrderStatus atomically moves an order from fromStatus to toStatus for a specific owner.
// Returns true if the row was updated (transition succeeded), false if no matching row was found
// (order doesn't exist, wrong owner, or already in a different status — i.e. another goroutine won).
func (s *OrderStore) TransitionOrderStatus(ctx context.Context, orderID string, clientID int, fromStatus, toStatus string) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE grocery_orders SET status = $1 WHERE order_id = $2 AND client_id = $3 AND status = $4`,
		toStatus, orderID, clientID, fromStatus,
	)
	if err != nil {
		return false, fmt.Errorf("failed to transition order status: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}
