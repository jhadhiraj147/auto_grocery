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

func NewMemoryStore(addr string, password string, db int) *MemoryStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &MemoryStore{client: rdb}
}

func (m *MemoryStore) SaveOrderItems(ctx context.Context, orderID string, items map[string]int32) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return m.client.Set(ctx, "order:"+orderID+":items", data, 1*time.Hour).Err()
}

func (m *MemoryStore) GetOrderItems(ctx context.Context, orderID string) (map[string]int32, error) {
	val, err := m.client.Get(ctx, "order:"+orderID+":items").Result()
	if err != nil {
		return nil, err
	}

	var items map[string]int32
	err = json.Unmarshal([]byte(val), &items)
	return items, nil
}

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

func (m *MemoryStore) DeleteOrderData(ctx context.Context, orderID string) {
	m.client.Del(ctx, "order:"+orderID+":items", "order:"+orderID+":count")
}
