package mocks

import (
	"context"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

// LockerStub implements LockService interface
type LockerStub struct {
	IsEventProcessedCalled func(ctx context.Context, blockHash string) (bool, error)
	HasConnectionCalled    func(ctx context.Context) bool
}

// IsEventProcessed -
func (ls *LockerStub) IsEventProcessed(ctx context.Context, blockHash string) (bool, error) {
	if ls.IsEventProcessedCalled != nil {
		return ls.IsEventProcessedCalled(ctx, blockHash)
	}

	return false, nil
}

func (ls *LockerStub) IsCrossShardConfirmation(ctx context.Context, originalTxHash string, event *transaction.Event) (bool, error) {

	return false, nil
}

// HasConnection -
func (ls *LockerStub) HasConnection(ctx context.Context) bool {
	if ls.HasConnectionCalled != nil {
		return ls.HasConnectionCalled(ctx)
	}

	return false
}

// IsInterfaceNil -
func (ls *LockerStub) IsInterfaceNil() bool {
	return ls == nil
}
