package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/multiversx/mx-chain-core-go/core/check"
	"github.com/truststaking/mx-chain-notifier-go/data"
)

type ArgsRedlockWrapper struct {
	Client       RedLockClient
	TTLInMinutes uint32
}

type redlockWrapper struct {
	client RedLockClient
	ttl    time.Duration
}

// NewRedlockWrapper create a new redLock based on a cache instance
func NewRedlockWrapper(args ArgsRedlockWrapper) (*redlockWrapper, error) {
	if check.IfNil(args.Client) {
		return nil, ErrNilRedlockClient
	}
	if args.TTLInMinutes == 0 {
		return nil, fmt.Errorf("%w for TTL in minutes", ErrZeroValueReceived)
	}

	ttl := time.Minute * time.Duration(args.TTLInMinutes)

	return &redlockWrapper{
		client: args.Client,
		ttl:    ttl,
	}, nil
}

// IsEventProcessed returns wether the item is already locked
func (r *redlockWrapper) IsEventProcessed(ctx context.Context, blockHash string) (bool, error) {
	return r.client.SetEntry(ctx, blockHash, true, r.ttl)
}

func (r *redlockWrapper) IsCrossShardConfirmation(ctx context.Context, originalTxHash string, event data.EventDuplicateCheck) (bool, error) {
	jsonData, err := json.Marshal(event)
	if err != nil {
		return false, err
	}
	log.Info("originalTxHash", "originalTxHash", originalTxHash)
	eventExists, err := r.client.HasEvent(ctx, originalTxHash, jsonData)
	if err != nil {
		return false, err
	}
	if eventExists {
		log.Info("event already exists", "event", jsonData)
		return true, nil
	}

	_, err = r.client.AddEventToList(ctx, originalTxHash, jsonData, time.Minute)
	if err != nil {
		return false, err
	}
	log.Info("added first entry", "event", jsonData)
	return false, nil

}

// HasConnection returns true if the redis client is connected
func (r *redlockWrapper) HasConnection(ctx context.Context) bool {
	return r.client.IsConnected(ctx)
}

// IsInterfaceNil returns true if there is no value under the interface
func (r *redlockWrapper) IsInterfaceNil() bool {
	return r == nil
}
