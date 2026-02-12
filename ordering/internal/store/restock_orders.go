package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type RestockOrder struct {
	ID        int
	OrderID   string
	TruckID   int
	Status    string
	CreatedAt time.Time
}

type RestockOrderItem struct {
	ID       int
	OrderID  int
	Sku      string
	Quantity int
}

type RestockStore struct {
	db *sql.DB
}

func NewRestockStore(db *sql.DB) *RestockStore {
	return &RestockStore{db: db}
}

func (s *RestockStore) CreateRestockOrder(ctx context.Context, order RestockOrder, items []RestockOrderItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queryHeader := `
		INSERT INTO restock_orders (order_id, truck_id, status)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	err = tx.QueryRowContext(ctx, queryHeader, order.OrderID, order.TruckID, "PENDING").Scan(&order.ID)
	if err != nil {
		return fmt.Errorf("failed to save restock header: %w", err)
	}

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

	return tx.Commit()
}

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
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get last restock: %w", err)
	}

	o.TruckID = truckID
	return &o, nil
}
