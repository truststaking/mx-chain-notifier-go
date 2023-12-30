package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

// RedisClientMock -
type RedisClientMock struct {
	mut     sync.Mutex
	entries map[string]bool
}

// NewRedisClientMock -
func NewRedisClientMock() *RedisClientMock {
	return &RedisClientMock{
		entries: make(map[string]bool),
	}
}

// SetEntry -
func (rc *RedisClientMock) SetEntry(_ context.Context, key string, value bool, ttl time.Duration) (bool, error) {
	rc.mut.Lock()
	defer rc.mut.Unlock()

	willSet := true
	for k, val := range rc.entries {
		if k == key && val == value {
			willSet = false
			break
		}
	}

	if willSet {
		rc.entries[key] = value
		return true, nil
	}

	return false, nil
}

// SetEntry will try to update a key value entry in redis database
func (rc *RedisClientMock) AddEventToList(ctx context.Context, key string, value *transaction.Event, ttl time.Duration) (int64, error) {
	rc.mut.Lock()
	defer rc.mut.Unlock()
	return 1, nil
}

// GetEventList will try to get the list of events from redis database
func (rc *RedisClientMock) GetEventList(ctx context.Context, key string) ([]string, error) {
	rc.mut.Lock()
	defer rc.mut.Unlock()
	return make([]string, 0), nil
}

// GetEntries -
func (rc *RedisClientMock) GetEntries() map[string]bool {
	rc.mut.Lock()
	defer rc.mut.Unlock()

	return rc.entries
}

// Ping -
func (rc *RedisClientMock) Ping(_ context.Context) (string, error) {
	return "PONG", nil
}

// IsConnected -
func (rc *RedisClientMock) IsConnected(_ context.Context) bool {
	return true
}

// IsInterfaceNil -
func (rc *RedisClientMock) IsInterfaceNil() bool {
	return rc == nil
}
