package mocks

import (
	"github.com/truststaking/mx-chain-notifier-go/data"
)

// EventsHandlerStub implements EventsHandler interface
type EventsHandlerStub struct {
	SkipRecivedDuplicatedEventsCalled func(id, value string)
	HandlePushEventsCalled            func(events data.BlockEvents) error
	HandleRevertEventsCalled          func(revertBlock data.RevertBlock)
	HandleFinalizedEventsCalled       func(finalizedBlock data.FinalizedBlock)
	HandleBlockTxsCalled              func(blockTxs data.BlockTxs)
	HandleBlockScrsCalled             func(blockScrs data.BlockScrs)
	HandleBlockEventsWithOrderCalled  func(blockTxs data.BlockEventsWithOrder)
}

// SkipRecivedDuplicatedEvents -
func (e *EventsHandlerStub) SkipRecivedDuplicatedEvents(id, value string) bool {
	return true
}

// HandleAlteredAccounts -
func (e *EventsHandlerStub) HandleAlteredAccounts(accounts data.AlteredAccountsEvent) {
}

// HandlePushEvents -
func (e *EventsHandlerStub) HandlePushEvents(events data.BlockEvents) error {
	if e.HandlePushEventsCalled != nil {
		return e.HandlePushEventsCalled(events)
	}

	return nil
}

// HandleRevertEvents -
func (e *EventsHandlerStub) HandleRevertEvents(revertBlock data.RevertBlock) {
	if e.HandleRevertEventsCalled != nil {
		e.HandleRevertEventsCalled(revertBlock)
	}
}

// HandleFinalizedEvents -
func (e *EventsHandlerStub) HandleFinalizedEvents(finalizedBlock data.FinalizedBlock) {
	if e.HandleFinalizedEventsCalled != nil {
		e.HandleFinalizedEventsCalled(finalizedBlock)
	}
}

// HandleBlockTxs -
func (e *EventsHandlerStub) HandleBlockTxs(blockTxs data.BlockTxs) {
	if e.HandleBlockTxsCalled != nil {
		e.HandleBlockTxsCalled(blockTxs)
	}
}

// HandleBlockScrs -
func (e *EventsHandlerStub) HandleBlockScrs(blockScrs data.BlockScrs) {
	if e.HandleBlockScrsCalled != nil {
		e.HandleBlockScrsCalled(blockScrs)
	}
}

// HandleBlockEventsWithOrder -
func (e *EventsHandlerStub) HandleBlockEventsWithOrder(blockTxs data.BlockEventsWithOrder) {
	if e.HandleBlockEventsWithOrderCalled != nil {
		e.HandleBlockEventsWithOrderCalled(blockTxs)
	}
}

// IsInterfaceNil -
func (e *EventsHandlerStub) IsInterfaceNil() bool {
	return e == nil
}
