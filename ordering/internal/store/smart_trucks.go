package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Matches the 'smart_trucks' table
type SmartTruck struct {
	ID          int
	TruckID     string // Unique ID (e.g. "TRUCK-55-B")
	PlateNumber string
	DriverName  string
	ContactInfo string
	Location    string
}

type TruckStore struct {
	db *sql.DB
}

func NewTruckStore(db *sql.DB) *TruckStore {
	return &TruckStore{db: db}
}

// CreateSmartTruck saves a new supplier truck
func (s *TruckStore) CreateSmartTruck(ctx context.Context, t SmartTruck) error {
	query := `
		INSERT INTO smart_trucks (truck_id, plate_number, driver_name, contact_info, location)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err := s.db.QueryRowContext(ctx, query, 
		t.TruckID, 
		t.PlateNumber, 
		t.DriverName, 
		t.ContactInfo, 
		t.Location,
	).Scan(&t.ID)

	if err != nil {
		return fmt.Errorf("failed to register smart truck: %w", err)
	}
	return nil
}

// GetSmartTruck finds a truck (useful for Restock validation later)
func (s *TruckStore) GetSmartTruck(ctx context.Context, truckID string) (*SmartTruck, error) {
	query := `
		SELECT id, truck_id, plate_number, driver_name, contact_info, location
		FROM smart_trucks
		WHERE truck_id = $1
	`
	var t SmartTruck
	err := s.db.QueryRowContext(ctx, query, truckID).Scan(
		&t.ID, &t.TruckID, &t.PlateNumber, &t.DriverName, &t.ContactInfo, &t.Location,
	)
	if err != nil {
		return nil, fmt.Errorf("truck not found: %w", err)
	}
	return &t, nil
}