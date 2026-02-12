package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type SmartClient struct {
	ID           int
	DeviceID     string
	Email        string
	Phone        string
	PasswordHash string
	CardInfoEnc  string
	RefreshToken string
	TokenExpiry  time.Time
}

type ClientStore struct {
	db *sql.DB
}

func NewClientStore(db *sql.DB) *ClientStore {
	return &ClientStore{db: db}
}

func (s *ClientStore) CreateSmartClient(ctx context.Context, c SmartClient) error {
	query := `
		INSERT INTO smart_clients (device_id, email, phone, password_hash, card_info_enc)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err := s.db.QueryRowContext(ctx, query,
		c.DeviceID,
		c.Email,
		c.Phone,
		c.PasswordHash,
		c.CardInfoEnc,
	).Scan(&c.ID)

	if err != nil {
		return fmt.Errorf("failed to register smart client: %w", err)
	}
	return nil
}

func (s *ClientStore) GetSmartClient(ctx context.Context, deviceID string) (*SmartClient, error) {
	query := `
		SELECT id, device_id, email, password_hash, card_info_enc, refresh_token, token_expiry
		FROM smart_clients
		WHERE device_id = $1
	`
	var c SmartClient

	var expiry sql.NullTime
	var token sql.NullString

	err := s.db.QueryRowContext(ctx, query, deviceID).Scan(
		&c.ID, &c.DeviceID, &c.Email, &c.PasswordHash, &c.CardInfoEnc, &token, &expiry,
	)

	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}

	if token.Valid {
		c.RefreshToken = token.String
	}
	if expiry.Valid {
		c.TokenExpiry = expiry.Time
	}

	return &c, nil
}

func (s *ClientStore) SetRefreshToken(ctx context.Context, deviceID string, token string, expiry time.Time) error {
	query := `
		UPDATE smart_clients
		SET refresh_token = $1, token_expiry = $2
		WHERE device_id = $3
	`
	_, err := s.db.ExecContext(ctx, query, token, expiry, deviceID)
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}
	return nil
}
