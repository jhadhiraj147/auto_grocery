package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type GroceryOrder struct {
	ID         int
	OrderID    string
	ClientID   int
	Status     string
	TotalPrice float64
	CreatedAt  time.Time
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

func NewOrderStore(db *sql.DB) *OrderStore {
	return &OrderStore{db: db}
}

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
		if err := rows.Scan(&o.ID, &o.OrderID, &o.Status, &o.TotalPrice, &o.CreatedAt); err != nil {
			return nil, err
		}
		o.ClientID = clientID
		history = append(history, o)
	}

	return history, nil
}

func (s *OrderStore) GetLastOrderByClientID(ctx context.Context, clientID int) (*GroceryOrder, error) {
	query := `
		SELECT id, order_id, status, total_price, created_at
		FROM grocery_orders
		WHERE client_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var o GroceryOrder

	err := s.db.QueryRowContext(ctx, query, clientID).Scan(
		&o.ID, &o.OrderID, &o.Status, &o.TotalPrice, &o.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get last order: %w", err)
	}

	o.ClientID = clientID
	return &o, nil
}

func (s *OrderStore) UpdateOrderStatus(ctx context.Context, orderID string, status string, totalPrice float64) error {
	// We add ::NUMERIC to explicitly tell Postgres how to handle $2
	query := `
		UPDATE grocery_orders
		SET status = $1, 
		    total_price = CASE WHEN $2::NUMERIC > 0 THEN $2::NUMERIC ELSE total_price END
		WHERE order_id = $3
	`

	// DOUBLE CHECK THIS ORDER: 1=status, 2=price, 3=id
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

func (s *OrderStore) GetOrderItems(ctx context.Context, orderID string) ([]GroceryOrderItem, error) {
	var dbID int
	// Convert UUID (string) to Internal ID (int)
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

func (s *OrderStore) DeleteOrder(ctx context.Context, orderID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var dbID int
	err = tx.QueryRowContext(ctx, "SELECT id FROM grocery_orders WHERE order_id = $1", orderID).Scan(&dbID)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM grocery_order_items WHERE order_id = $1", dbID)
	if err != nil {
		return fmt.Errorf("failed to delete items: %w", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM grocery_orders WHERE id = $1", dbID)
	if err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}

	return tx.Commit()
}

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

	// 🔧 THE CORE FIX: 
	// The DB time is retrieved as UTC but contains Local hours.
	// We must strip the UTC label and replace it with Local.
	o.CreatedAt = time.Date(
		o.CreatedAt.Year(), o.CreatedAt.Month(), o.CreatedAt.Day(),
		o.CreatedAt.Hour(), o.CreatedAt.Minute(), o.CreatedAt.Second(), 
		o.CreatedAt.Nanosecond(), time.Local,
	)

	return &o, nil
}

func (s *OrderStore) UpdateStatus(ctx context.Context, orderID string, status string) error {
	query := `UPDATE grocery_orders SET status = $1 WHERE order_id = $2`
	_, err := s.db.ExecContext(ctx, query, status, orderID)
	return err
}