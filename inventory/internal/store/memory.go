package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type MemoryStore struct {
	client *redis.Client
}

// NewMemoryStore now takes addr, password, and db index from your .env
func NewMemoryStore(addr string, password string, db int) *MemoryStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,     // From REDIS_ADDR
		Password: password, // From REDIS_PW
		DB:       db,       // From REDIS_DB
	})
	return &MemoryStore{client: rdb}
}

// 1. Save the Items (So we remember what to bill later)
func (m *MemoryStore) SaveOrderItems(ctx context.Context, orderID string, items map[string]int32) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	// Save with a 1-hour expiration (cleanup automatically)
	return m.client.Set(ctx, "order:"+orderID+":items", data, 1*time.Hour).Err()
}

// 2. Get the Items (When the robots are done)
func (m *MemoryStore) GetOrderItems(ctx context.Context, orderID string) (map[string]int32, error) {
	val, err := m.client.Get(ctx, "order:"+orderID+":items").Result()
	if err != nil {
		return nil, err
	}

	var items map[string]int32
	err = json.Unmarshal([]byte(val), &items)
	return items, nil
}

// 3. Increment Robot Count (Atomic +1)
func (m *MemoryStore) IncrementRobotCount(ctx context.Context, orderID string) (int64, error) {
	key := "order:" + orderID + ":count"
	
	count, err := m.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	
	if count == 1 {
		m.client.Expire(ctx, key, 1*time.Hour)
	}
	
	return count, nil
}

// 4. Cleanup (Delete keys after the order is finalized)
func (m *MemoryStore) DeleteOrderData(ctx context.Context, orderID string) {
	m.client.Del(ctx, "order:"+orderID+":items", "order:"+orderID+":count")
}