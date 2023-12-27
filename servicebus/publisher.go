package servicebus

import (
	"context"

	"github.com/multiversx/mx-chain-core-go/core/check"
	"github.com/multiversx/mx-chain-core-go/marshal"
	logger "github.com/multiversx/mx-chain-logger-go"
	"github.com/multiversx/mx-chain-notifier-go/common"
	"github.com/multiversx/mx-chain-notifier-go/config"
	"github.com/multiversx/mx-chain-notifier-go/data"
)

var log = logger.GetOrCreate("servicebus")

// ArgsServiceBusPublisher defines the arguments needed for rabbitmq publisher creation
type ArgsServiceBusPublisher struct {
	Client     ServiceBusClient
	Config     config.AzureServiceBusConfig
	Marshaller marshal.Marshalizer
}

type serviceBusPublisher struct {
	client     ServiceBusClient
	marshaller marshal.Marshalizer
	cfg        config.AzureServiceBusConfig

	broadcast                     chan data.BlockEvents
	broadcastRevert               chan data.RevertBlock
	broadcastFinalized            chan data.FinalizedBlock
	broadcastTxs                  chan data.BlockTxs
	broadcastBlockEventsWithOrder chan data.BlockEventsWithOrder
	broadcastScrs                 chan data.BlockScrs

	cancelFunc func()
	closeChan  chan struct{}
}

// NewServiceBusPublisher creates a new rabbitMQ publisher instance
func NewServiceBusPublisher(args ArgsServiceBusPublisher) (*serviceBusPublisher, error) {
	err := checkArgs(args)
	if err != nil {
		return nil, err
	}

	sb := &serviceBusPublisher{
		broadcast:                     make(chan data.BlockEvents),
		broadcastRevert:               make(chan data.RevertBlock),
		broadcastFinalized:            make(chan data.FinalizedBlock),
		broadcastTxs:                  make(chan data.BlockTxs),
		broadcastScrs:                 make(chan data.BlockScrs),
		broadcastBlockEventsWithOrder: make(chan data.BlockEventsWithOrder),
		cfg:                           args.Config,
		client:                        args.Client,
		marshaller:                    args.Marshaller,
		closeChan:                     make(chan struct{}),
	}

	return sb, nil
}

func checkArgs(args ArgsServiceBusPublisher) error {
	if check.IfNil(args.Client) {
		return ErrNilServiceBusClient
	}
	if check.IfNil(args.Marshaller) {
		return common.ErrNilMarshaller
	}

	if args.Config.EventsExchange.Name == "" {
		return ErrInvalidServiceBusExchangeName
	}

	if args.Config.RevertEventsExchange.Name == "" {
		return ErrInvalidServiceBusExchangeName
	}
	if args.Config.FinalizedEventsExchange.Name == "" {
		return ErrInvalidServiceBusExchangeName
	}
	if args.Config.BlockTxsExchange.Name == "" {
		return ErrInvalidServiceBusExchangeName
	}
	if args.Config.BlockScrsExchange.Name == "" {
		return ErrInvalidServiceBusExchangeName
	}
	if args.Config.BlockEventsExchange.Name == "" {
		return ErrInvalidServiceBusExchangeName
	}

	return nil
}

// Run is launched as a goroutine and listens for events on the exposed channels
func (sb *serviceBusPublisher) Run() {
	var ctx context.Context
	ctx, sb.cancelFunc = context.WithCancel(context.Background())

	go sb.run(ctx)
}

func (sb *serviceBusPublisher) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debug("ServiceBus publisher is stopping...")
			sb.client.Close()
		case events := <-sb.broadcast:
			sb.publishToExchanges(events)
		case revertBlock := <-sb.broadcastRevert:
			sb.publishRevertToExchange(revertBlock)
		case finalizedBlock := <-sb.broadcastFinalized:
			sb.publishFinalizedToExchange(finalizedBlock)
		case blockTxs := <-sb.broadcastTxs:
			sb.publishTxsToExchange(blockTxs)
		case blockScrs := <-sb.broadcastScrs:
			sb.publishScrsToExchange(blockScrs)
		case blockEvents := <-sb.broadcastBlockEventsWithOrder:
			sb.publishBlockEventsWithOrderToExchange(blockEvents)
		}
	}
}

// Broadcast will handle the block events pushed by producers and sends them to rabbitMQ channel
func (sb *serviceBusPublisher) Broadcast(events data.BlockEvents) {
	select {
	case sb.broadcast <- events:
	case <-sb.closeChan:
	}
}

// BroadcastRevert will handle the revert event pushed by producers and sends them to rabbitMQ channel
func (sb *serviceBusPublisher) BroadcastRevert(events data.RevertBlock) {
	select {
	case sb.broadcastRevert <- events:
	case <-sb.closeChan:
	}
}

// BroadcastFinalized will handle the finalized event pushed by producers and sends them to rabbitMQ channel
func (sb *serviceBusPublisher) BroadcastFinalized(events data.FinalizedBlock) {
	select {
	case sb.broadcastFinalized <- events:
	case <-sb.closeChan:
	}
}

// BroadcastTxs will handle the txs event pushed by producers and sends them to rabbitMQ channel
func (sb *serviceBusPublisher) BroadcastTxs(events data.BlockTxs) {
	select {
	case sb.broadcastTxs <- events:
	case <-sb.closeChan:
	}
}

// BroadcastScrs will handle the scrs event pushed by producers and sends them to rabbitMQ channel
func (sb *serviceBusPublisher) BroadcastScrs(events data.BlockScrs) {
	select {
	case sb.broadcastScrs <- events:
	case <-sb.closeChan:
	}
}

// BroadcastBlockEventsWithOrder will handle the full block events pushed by producers and sends them to rabbitMQ channel
func (sb *serviceBusPublisher) BroadcastBlockEventsWithOrder(events data.BlockEventsWithOrder) {
	select {
	case sb.broadcastBlockEventsWithOrder <- events:
	case <-sb.closeChan:
	}
}

func (sb *serviceBusPublisher) publishToExchanges(events data.BlockEvents) {
	eventsBytes, err := sb.marshaller.Marshal(events)
	if err != nil {
		log.Error("could not marshal events", "err", err.Error())
		return
	}

	err = sb.publishFanout(sb.cfg.EventsExchange, eventsBytes)
	if err != nil {
		log.Error("failed to publish events to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishRevertToExchange(revertBlock data.RevertBlock) {
	revertBlockBytes, err := sb.marshaller.Marshal(revertBlock)
	if err != nil {
		log.Error("could not marshal revert event", "err", err.Error())
		return
	}

	err = sb.publishFanout(sb.cfg.RevertEventsExchange, revertBlockBytes)
	if err != nil {
		log.Error("failed to publish revert event to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishFinalizedToExchange(finalizedBlock data.FinalizedBlock) {
	finalizedBlockBytes, err := sb.marshaller.Marshal(finalizedBlock)
	if err != nil {
		log.Error("could not marshal finalized event", "err", err.Error())
		return
	}

	err = sb.publishFanout(sb.cfg.FinalizedEventsExchange, finalizedBlockBytes)
	if err != nil {
		log.Error("failed to publish finalized event to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishTxsToExchange(blockTxs data.BlockTxs) {
	txsBlockBytes, err := sb.marshaller.Marshal(blockTxs)
	if err != nil {
		log.Error("could not marshal block txs event", "err", err.Error())
		return
	}

	err = sb.publishFanout(sb.cfg.BlockTxsExchange, txsBlockBytes)
	if err != nil {
		log.Error("failed to publish block txs event to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishScrsToExchange(blockScrs data.BlockScrs) {
	scrsBlockBytes, err := sb.marshaller.Marshal(blockScrs)
	if err != nil {
		log.Error("could not marshal block scrs event", "err", err.Error())
		return
	}

	err = sb.publishFanout(sb.cfg.BlockScrsExchange, scrsBlockBytes)
	if err != nil {
		log.Error("failed to publish block scrs event to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishBlockEventsWithOrderToExchange(blockTxs data.BlockEventsWithOrder) {
	txsBlockBytes, err := sb.marshaller.Marshal(blockTxs)
	if err != nil {
		log.Error("could not marshal block txs event", "err", err.Error())
		return
	}

	err = sb.publishFanout(sb.cfg.BlockEventsExchange, txsBlockBytes)
	if err != nil {
		log.Error("failed to publish full block events to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishFanout(exchangeConfig config.ServiceBusExchangeConfig, payload []byte) error {
	return sb.client.Publish(exchangeConfig, sb.cfg, payload)
}

// Close will close the channels
func (sb *serviceBusPublisher) Close() error {
	if sb.cancelFunc != nil {
		sb.cancelFunc()
	}

	close(sb.closeChan)

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (sb *serviceBusPublisher) IsInterfaceNil() bool {
	return sb == nil
}
