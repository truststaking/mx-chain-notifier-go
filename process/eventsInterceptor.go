package process

import (
	"encoding/hex"

	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-core-go/core/check"
	"github.com/multiversx/mx-chain-core-go/data/outport"
	"github.com/multiversx/mx-chain-core-go/data/smartContractResult"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/truststaking/mx-chain-notifier-go/data"
)

// logEvent defines a log event associated with corresponding tx hash
type logEvent struct {
	EventHandler   *transaction.Event
	TxHash         string
	OriginalTxHash string
}

// ArgsEventsInterceptor defines the arguments needed for creating an events interceptor instance
type ArgsEventsInterceptor struct {
	PubKeyConverter core.PubkeyConverter
}

type eventsInterceptor struct {
	pubKeyConverter core.PubkeyConverter
}

// NewEventsInterceptor creates a new eventsInterceptor instance
func NewEventsInterceptor(args ArgsEventsInterceptor) (*eventsInterceptor, error) {
	if check.IfNil(args.PubKeyConverter) {
		return nil, ErrNilPubKeyConverter
	}

	return &eventsInterceptor{
		pubKeyConverter: args.PubKeyConverter,
	}, nil
}

// ProcessBlockEvents will process block events data
func (ei *eventsInterceptor) ProcessBlockEvents(eventsData *data.ArgsSaveBlockData) (*data.InterceptorBlockData, error) {
	if eventsData == nil {
		return nil, ErrNilBlockEvents
	}
	if eventsData.TransactionsPool == nil {
		return nil, ErrNilTransactionsPool
	}
	if eventsData.Body == nil {
		return nil, ErrNilBlockBody
	}
	if eventsData.Header == nil {
		return nil, ErrNilBlockHeader
	}

	scrs := make(map[string]*smartContractResult.SmartContractResult)
	for hash, scr := range eventsData.TransactionsPool.SmartContractResults {
		scrs[hash] = scr.SmartContractResult
	}
	scrsWithOrder := eventsData.TransactionsPool.SmartContractResults

	events := ei.getLogEventsFromTransactionsPool(eventsData.TransactionsPool.Logs, scrs)

	txs := make(map[string]*transaction.Transaction)
	for hash, tx := range eventsData.TransactionsPool.Transactions {
		txs[hash] = tx.Transaction
	}
	txsWithOrder := eventsData.TransactionsPool.Transactions

	return &data.InterceptorBlockData{
		Hash:          hex.EncodeToString(eventsData.HeaderHash),
		Body:          eventsData.Body,
		Header:        eventsData.Header,
		Txs:           txs,
		TxsWithOrder:  txsWithOrder,
		Scrs:          scrs,
		ScrsWithOrder: scrsWithOrder,
		LogEvents:     events,
	}, nil
}

func (ei *eventsInterceptor) getLogEventsFromTransactionsPool(logs []*outport.LogData, scrs map[string]*smartContractResult.SmartContractResult) []data.Event {
	var logEvents []*logEvent
	for _, logData := range logs {
		if logData == nil {
			continue
		}
		if check.IfNilReflect(logData.Log) {
			continue
		}
		var tmpLogEvents []*logEvent
		skipTransfers := false
		for _, event := range logData.Log.Events {
			eventIdentifier := string(event.Identifier)
			originalTxHash := logData.TxHash
			if eventIdentifier == core.SignalErrorOperation || eventIdentifier == core.InternalVMErrorsOperation {
				scResult, exists := scrs[logData.TxHash]

				if !exists {
					skipTransfers = true
				} else {
					originalTxHash = string(scResult.OriginalTxHash)
				}
				log.Debug("eventsInterceptor: received signalError or internalVMErrors event from log event",
					"txHash", logData.TxHash,
					"isSCResult", exists,
					"skipTransfers", skipTransfers,
				)
			}
			le := &logEvent{
				EventHandler:   event,
				TxHash:         logData.TxHash,
				OriginalTxHash: originalTxHash,
			}

			tmpLogEvents = append(tmpLogEvents, le)
		}
		if skipTransfers {
			var filteredItems []*logEvent
			for _, item := range tmpLogEvents {
				identifier := string(item.EventHandler.GetIdentifier())
				if identifier == core.BuiltInFunctionMultiESDTNFTTransfer || identifier == core.BuiltInFunctionESDTNFTTransfer || identifier == core.BuiltInFunctionESDTTransfer {
					continue
				}
				filteredItems = append(filteredItems, item)
			}
			logEvents = append(logEvents, filteredItems...)
		} else {
			logEvents = append(logEvents, tmpLogEvents...)
		}
	}

	if len(logEvents) == 0 {
		return nil
	}
	uniqueEvents := uniqueLogEvents(logEvents)
	if len(uniqueEvents) == 0 {
		return nil
	}
	events := make([]data.Event, 0, len(uniqueEvents))
	for _, event := range uniqueEvents {
		if event == nil || check.IfNil(event.EventHandler) {
			continue
		}
		bech32Address, err := ei.pubKeyConverter.Encode(event.EventHandler.GetAddress())
		if err != nil {
			log.Error("eventsInterceptor: failed to decode event address", "error", err)
			continue
		}
		eventIdentifier := string(event.EventHandler.GetIdentifier())

		log.Debug("eventsInterceptor: received event from address",
			"address", bech32Address,
			"identifier", eventIdentifier,
		)

		events = append(events, data.Event{
			Address:        bech32Address,
			Identifier:     eventIdentifier,
			Topics:         event.EventHandler.GetTopics(),
			Data:           event.EventHandler.GetData(),
			TxHash:         event.TxHash,
			OriginalTxHash: event.OriginalTxHash,
		})
	}

	return events
}

func uniqueLogEvents(events []*logEvent) []*logEvent {
	var unique []*logEvent
	for _, event := range events {
		isDuplicate := false
		for _, uniqueEvent := range unique {
			if event.EventHandler.Equal(uniqueEvent.EventHandler) && event.OriginalTxHash == uniqueEvent.OriginalTxHash {
				isDuplicate = true
				log.Info("eventsInterceptor: received duplicated event", "event", event.EventHandler.GoString(), "txHash", event.TxHash)
				break
			}
		}
		if !isDuplicate {
			unique = append(unique, event)
		}
	}

	return unique
}

// IsInterfaceNil returns whether the interface is nil
func (ei *eventsInterceptor) IsInterfaceNil() bool {
	return ei == nil
}
