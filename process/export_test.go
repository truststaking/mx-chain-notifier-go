package process

import (
	"github.com/multiversx/mx-chain-core-go/data/outport"
	"github.com/multiversx/mx-chain-core-go/data/smartContractResult"
	"github.com/truststaking/mx-chain-notifier-go/data"
)

// TryCheckProcessedWithRetry exports internal method for testing
func (eh *eventsHandler) TryCheckProcessedWithRetry(prefix, blockHash string) bool {
	return eh.tryCheckProcessedWithRetry(prefix, blockHash)
}

// GetLogEventsFromTransactionsPool exports internal method for testing
func (ei *eventsInterceptor) GetLogEventsFromTransactionsPool(logs []*outport.LogData) []data.Event {
	scrHashes := make(map[string]*smartContractResult.SmartContractResult)
	return ei.getLogEventsFromTransactionsPool(logs, scrHashes)
}
