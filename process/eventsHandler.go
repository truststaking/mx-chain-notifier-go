package process

import (
	"context"
	"fmt"
	"time"

	"github.com/multiversx/mx-chain-core-go/core/check"
	logger "github.com/multiversx/mx-chain-logger-go"
	"github.com/truststaking/mx-chain-notifier-go/common"
	"github.com/truststaking/mx-chain-notifier-go/data"
)

var log = logger.GetOrCreate("process")

const (
	setRetryDuration       = time.Millisecond * 500
	reconnectRetryDuration = time.Second * 2
	minRetries             = 1
	revertKeyPrefix        = "revert_"
	finalizedKeyPrefix     = "finalized_"
	txsKeyPrefix           = "txs_"
	txsWithOrderKeyPrefix  = "txsWithOrder_"
	scrsKeyPrefix          = "scrs_"

	rabbitmqMetricPrefix = "RabbitMQ"
	redisMetricPrefix    = "Redis"
)

// ArgsEventsHandler defines the arguments needed for an events handler
type ArgsEventsHandler struct {
	Locker               LockService
	Publisher            Publisher
	StatusMetricsHandler common.StatusMetricsHandler
	CheckDuplicates      bool
}

type eventsHandler struct {
	locker          LockService
	publisher       Publisher
	metricsHandler  common.StatusMetricsHandler
	checkDuplicates bool
}

// NewEventsHandler creates a new events handler component
func NewEventsHandler(args ArgsEventsHandler) (*eventsHandler, error) {
	err := checkArgs(args)
	if err != nil {
		return nil, err
	}

	return &eventsHandler{
		locker:          args.Locker,
		publisher:       args.Publisher,
		metricsHandler:  args.StatusMetricsHandler,
		checkDuplicates: args.CheckDuplicates,
	}, nil
}

func checkArgs(args ArgsEventsHandler) error {
	if check.IfNil(args.Locker) {
		return ErrNilLockService
	}
	if check.IfNil(args.Publisher) {
		return ErrNilPublisherService
	}
	if check.IfNil(args.StatusMetricsHandler) {
		return common.ErrNilStatusMetricsHandler
	}

	return nil
}

// HandlePushEvents will handle push events received from observer
func (eh *eventsHandler) HandlePushEvents(events data.BlockEvents) error {
	if events.Hash == "" {
		log.Debug("received empty hash", "event", common.PushLogsAndEvents,
			"will process", false,
		)
		return common.ErrReceivedEmptyEvents
	}

	if len(events.Events) == 0 {
		events.Events = make([]data.Event, 0)
	}

	t := time.Now()
	eh.publisher.Broadcast(events)
	eh.metricsHandler.AddRequest(getRabbitOpID(common.PushLogsAndEvents), time.Since(t))
	return nil
}

func (eh *eventsHandler) SkipRecivedDuplicatedEvents(id string, value string) bool {

	shouldProcessRevert := true
	if eh.checkDuplicates {
		shouldProcessRevert = eh.tryCheckProcessedWithRetry(id, value)
	}

	if !shouldProcessRevert {
		// log.Info("received duplicated events", "event", id,
		// 	"block hash", value,
		// 	"will process", false,
		// )
		return true
	}
	return false
}

func (eh *eventsHandler) HandleAlteredAccounts(accounts data.AlteredAccountsEvent) {
	t := time.Now()
	eh.publisher.BroadcastAlteredAccounts(accounts)
	eh.metricsHandler.AddRequest(getRabbitOpID(common.RevertBlockEvents), time.Since(t))
}

// HandleRevertEvents will handle revents events received from observer
func (eh *eventsHandler) HandleRevertEvents(revertBlock data.RevertBlock) {
	if revertBlock.Hash == "" {
		log.Warn("received empty hash", "event", common.RevertBlockEvents,
			"will process", false,
		)
		return
	}

	skip := eh.SkipRecivedDuplicatedEvents(common.RevertBlockEvents, revertBlock.Hash)
	if skip {
		return
	}

	t := time.Now()
	eh.publisher.BroadcastRevert(revertBlock)
	eh.metricsHandler.AddRequest(getRabbitOpID(common.RevertBlockEvents), time.Since(t))
}

// HandleFinalizedEvents will handle finalized events received from observer
func (eh *eventsHandler) HandleFinalizedEvents(finalizedBlock data.FinalizedBlock) {
	if finalizedBlock.Hash == "" {
		log.Warn("received empty hash", "event", common.FinalizedBlockEvents,
			"will process", false,
		)
		return
	}

	skip := eh.SkipRecivedDuplicatedEvents(common.FinalizedBlockEvents, finalizedBlock.Hash)
	if skip {
		return
	}

	t := time.Now()
	eh.publisher.BroadcastFinalized(finalizedBlock)
	eh.metricsHandler.AddRequest(getRabbitOpID(common.FinalizedBlockEvents), time.Since(t))
}

// HandleBlockTxs will handle txs events received from observer
func (eh *eventsHandler) HandleBlockTxs(blockTxs data.BlockTxs) {
	if blockTxs.Hash == "" {
		log.Warn("received empty hash", "event", common.BlockTxs,
			"will process", false,
		)
		return
	}

	t := time.Now()
	eh.publisher.BroadcastTxs(blockTxs)
	eh.metricsHandler.AddRequest(getRabbitOpID(common.BlockTxs), time.Since(t))
}

// HandleBlockScrs will handle scrs events received from observer
func (eh *eventsHandler) HandleBlockScrs(blockScrs data.BlockScrs) {
	if blockScrs.Hash == "" {
		log.Warn("received empty hash", "event", common.BlockScrs,
			"will process", false,
		)
		return
	}

	t := time.Now()
	eh.publisher.BroadcastScrs(blockScrs)
	eh.metricsHandler.AddRequest(getRabbitOpID(common.BlockScrs), time.Since(t))
}

// HandleBlockEventsWithOrder will handle full block events received from observer
func (eh *eventsHandler) HandleBlockEventsWithOrder(blockTxs data.BlockEventsWithOrder) {
	if blockTxs.Hash == "" {
		log.Warn("received empty hash", "event", common.BlockEvents,
			"will process", false,
		)
		return
	}

	t := time.Now()
	eh.publisher.BroadcastBlockEventsWithOrder(blockTxs)
	eh.metricsHandler.AddRequest(getRabbitOpID(common.BlockEvents), time.Since(t))
}

func (eh *eventsHandler) tryCheckProcessedWithRetry(id, blockHash string) bool {
	var err error
	var setSuccessful bool

	prefix := getPrefixLockerKey(id)
	key := prefix + blockHash

	for {
		t := time.Now()
		setSuccessful, err = eh.locker.IsEventProcessed(context.Background(), key)
		eh.metricsHandler.AddRequest(getRedisOpID(id), time.Since(t))

		if err == nil {
			break
		}

		log.Error("failed to check event in locker", "error", err.Error())
		if !eh.locker.HasConnection(context.Background()) {
			log.Error("failure connecting to locker service")

			time.Sleep(reconnectRetryDuration)
		} else {
			time.Sleep(setRetryDuration)
		}
	}

	// log.Debug("locker", "event", id, "block hash", blockHash, "succeeded", setSuccessful)

	return setSuccessful
}

func getPrefixLockerKey(id string) string {
	// keep this matching for backwards compatibility
	switch id {
	case common.PushLogsAndEvents:
		return ""
	case common.RevertBlockEvents:
		return revertKeyPrefix
	case common.FinalizedBlockEvents:
		return finalizedKeyPrefix
	case common.BlockTxs:
		return txsKeyPrefix
	case common.BlockScrs:
		return scrsKeyPrefix
	case common.BlockEvents:
		return txsWithOrderKeyPrefix
	}

	return ""
}

func getRabbitOpID(operation string) string {
	return fmt.Sprintf("%s-%s", rabbitmqMetricPrefix, operation)
}

func getRedisOpID(operation string) string {
	return fmt.Sprintf("%s-%s", redisMetricPrefix, operation)
}

// IsInterfaceNil returns true if there is no value under the interface
func (eh *eventsHandler) IsInterfaceNil() bool {
	return eh == nil
}
