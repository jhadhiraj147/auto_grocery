package store

import (
	"context"
	"database/sql"
	"fmt"
)

type SmartTruck struct {
	ID          int
	TruckID     string
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

// UpsertSmartTruck: If truck exists, update details. If new, create it.
func (s *TruckStore) UpsertSmartTruck(ctx context.Context, t SmartTruck) (int, error) {
	// PostgreSql UPSERT syntax
	query := `
		INSERT INTO smart_trucks (truck_id, plate_number, driver_name, contact_info, location)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (truck_id) 
		DO UPDATE SET 
			plate_number = EXCLUDED.plate_number,
			driver_name = EXCLUDED.driver_name,
			contact_info = EXCLUDED.contact_info,
			location = EXCLUDED.location
		RETURNING id
	`
	var dbID int
	err := s.db.QueryRowContext(ctx, query, 
		t.TruckID, 
		t.PlateNumber, 
		t.DriverName, 
		t.ContactInfo, 
		t.Location,
	).Scan(&dbID)

	if err != nil {
		return 0, fmt.Errorf("failed to upsert truck: %w", err)
	}
	return dbID, nil
}

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