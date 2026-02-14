package store

import (
	"context"
	"encoding/json"
	"time"

	pb "auto_grocery/inventory/proto"

	"github.com/redis/go-redis/v9"
)

type MemoryStore struct {
	clientClient  *redis.Client // Database 0 (Clients)
	restockClient *redis.Client // Database 1 (Restocks)
}

// NewMemoryStore creates redis clients for client and restock data partitions.
func NewMemoryStore(addr string, password string) *MemoryStore {
	return &MemoryStore{
		clientClient: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       0,
		}),
		restockClient: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       1,
		}),
	}
}

// Ping verifies connectivity and credentials for both redis databases.
func (m *MemoryStore) Ping(ctx context.Context) error {
	if err := m.clientClient.Ping(ctx).Err(); err != nil {
		return err
	}
	if err := m.restockClient.Ping(ctx).Err(); err != nil {
		return err
	}
	return nil
}

// --- Client Methods (DB 0) ---

// SaveOrderItems stores client-order items in redis with a bounded ttl.
func (m *MemoryStore) SaveOrderItems(ctx context.Context, orderID string, items map[string]int32) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	// Key: orderID:items
	return m.clientClient.Set(ctx, orderID+":items", data, 1*time.Hour).Err()
}

// GetOrderItems loads client-order items from redis.
func (m *MemoryStore) GetOrderItems(ctx context.Context, orderID string) (map[string]int32, error) {
	val, err := m.clientClient.Get(ctx, orderID+":items").Result()
	if err != nil {
		return nil, err
	}
	var items map[string]int32
	err = json.Unmarshal([]byte(val), &items)
	return items, err
}

// IncrementClientRobotCount increments completed robot callbacks for client orders.
func (m *MemoryStore) IncrementClientRobotCount(ctx context.Context, orderID string) (int64, error) {
	key := orderID + ":count"
	return m.clientClient.Incr(ctx, key).Result()
}

// --- Restock Methods (DB 1) ---

// SaveRestockItems stores restock-order items in redis with a bounded ttl.
func (m *MemoryStore) SaveRestockItems(ctx context.Context, orderID string, items []*pb.RestockItem) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	// Key: orderID:items
	return m.restockClient.Set(ctx, orderID+":items", data, 1*time.Hour).Err()
}

// GetRestockItems loads restock-order items from redis.
func (m *MemoryStore) GetRestockItems(ctx context.Context, orderID string) ([]*pb.RestockItem, error) {
	val, err := m.restockClient.Get(ctx, orderID+":items").Result()
	if err != nil {
		return nil, err
	}
	var items []*pb.RestockItem
	err = json.Unmarshal([]byte(val), &items)
	return items, err
}

// IncrementRestockRobotCount increments completed robot callbacks for restock orders.
func (m *MemoryStore) IncrementRestockRobotCount(ctx context.Context, orderID string) (int64, error) {
	key := orderID + ":count"
	return m.restockClient.Incr(ctx, key).Result()
}

// --- Lifecycle Management ---

// DeleteOrderData clears transient redis keys for either order flow.
func (m *MemoryStore) DeleteOrderData(ctx context.Context, orderID string, isRestock bool) {
	if isRestock {
		m.restockClient.Del(ctx, orderID+":items", orderID+":count", orderID+":finalized")
	} else {
		m.clientClient.Del(ctx, orderID+":items", orderID+":count", orderID+":finalized")
	}
}

// TryMarkOrderFinalized sets a one-time finalize marker using SETNX semantics.
func (m *MemoryStore) TryMarkOrderFinalized(ctx context.Context, orderID string, isRestock bool) (bool, error) {
	key := orderID + ":finalized"
	if isRestock {
		return m.restockClient.SetNX(ctx, key, "1", 1*time.Hour).Result()
	}
	return m.clientClient.SetNX(ctx, key, "1", 1*time.Hour).Result()
}
