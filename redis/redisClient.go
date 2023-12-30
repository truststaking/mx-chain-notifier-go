package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

const (
	pongValue = "PONG"
)

type redisClientWrapper struct {
	redis *redis.Client
}

// NewRedisClientWrapper creates a wrapper over redis client instance. This is an abstraction
// over different redis connections: single instance, sentinel, cluster.
func NewRedisClientWrapper(redis *redis.Client) *redisClientWrapper {
	return &redisClientWrapper{
		redis: redis,
	}
}

// SetEntry will try to update a key value entry in redis database
func (rc *redisClientWrapper) SetEntry(ctx context.Context, key string, value bool, ttl time.Duration) (bool, error) {
	return rc.redis.SetNX(ctx, key, value, ttl).Result()
}

// SetEntry will try to update a key value entry in redis database
func (rc *redisClientWrapper) AddEventToList(ctx context.Context, key string, value *transaction.Event, ttl time.Duration) (int64, error) {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}

	index, err := rc.redis.RPushX(ctx, key, jsonData).Result()

	if err != nil {
		return 0, err
	}

	_, err = rc.redis.Expire(ctx, key, ttl).Result()
	if err != nil {
		return 0, err
	}

	return index, nil
}

// GetEventList will try to get the list of events from redis database
func (rc *redisClientWrapper) GetEventList(ctx context.Context, key string) ([]string, error) {
	return rc.redis.LRange(ctx, key, 0, -1).Result()
}

// Ping will check if Redis instance is reponding
func (rc *redisClientWrapper) Ping(ctx context.Context) (string, error) {
	return rc.redis.Ping(ctx).Result()
}

// IsConnected will check if Redis is connected
func (rc *redisClientWrapper) IsConnected(ctx context.Context) bool {
	pong, err := rc.Ping(context.Background())
	return err == nil && pong == pongValue
}

// IsInterfaceNil returns true if there is no value under the interface
func (rc *redisClientWrapper) IsInterfaceNil() bool {
	return rc == nil
}
