package servicebus

import (
	"context"
	"encoding/hex"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-core-go/core/check"
	"github.com/multiversx/mx-chain-core-go/marshal"
	logger "github.com/multiversx/mx-chain-logger-go"
	"github.com/truststaking/mx-chain-notifier-go/common"
	"github.com/truststaking/mx-chain-notifier-go/config"
	"github.com/truststaking/mx-chain-notifier-go/data"
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
	alteredAccount                chan data.AlteredAccountsEvent

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
		alteredAccount:                make(chan data.AlteredAccountsEvent),
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

	if args.Config.EventsExchange.Topic == "" {
		return ErrInvalidServiceBusExchangeName
	}
	if args.Config.RevertEventsExchange.Topic == "" {
		return ErrInvalidServiceBusExchangeName
	}
	if args.Config.FinalizedEventsExchange.Topic == "" {
		return ErrInvalidServiceBusExchangeName
	}
	if args.Config.BlockTxsExchange.Topic == "" {
		return ErrInvalidServiceBusExchangeName
	}
	if args.Config.BlockScrsExchange.Topic == "" {
		return ErrInvalidServiceBusExchangeName
	}
	if args.Config.BlockEventsExchange.Topic == "" {
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
		case alteredAccount := <-sb.alteredAccount:
			sb.publishAlteredAccounts(alteredAccount)
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

// Broadcast will handle the altered accounts pushed by producers and sends them to servicebus channel
func (sb *serviceBusPublisher) BroadcastAlteredAccounts(events data.AlteredAccountsEvent) {
	select {
	case sb.alteredAccount <- events:
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
	messages := make([]*azservicebus.Message, 0)

	for _, event := range events.Events {
		identifier := event.Identifier
		sessionId := event.Address
		isNFT := "true"

		if sb.cfg.SkipExecutionEventLogs {
			if identifier == core.WriteLogIdentifier ||
				identifier == core.SignalErrorOperation ||
				identifier == core.InternalVMErrorsOperation ||
				identifier == core.CompletedTxEventIdentifier {
				continue
			}
		}

		if identifier == core.BuiltInFunctionESDTNFTCreate ||
			identifier == core.BuiltInFunctionESDTNFTBurn ||
			identifier == core.BuiltInFunctionESDTNFTUpdateAttributes ||
			identifier == core.BuiltInFunctionESDTNFTAddURI ||
			identifier == core.BuiltInFunctionESDTNFTAddQuantity ||
			identifier == core.BuiltInFunctionMultiESDTNFTTransfer ||
			identifier == core.BuiltInFunctionESDTNFTTransfer ||
			identifier == core.BuiltInFunctionESDTTransfer {
			hexStr := hex.EncodeToString(event.Topics[1])
			if hexStr == "" {
				isNFT = "false"
			}
			sessionId = string(event.Topics[0])
		}

		payload, err := sb.marshaller.Marshal(event)
		if err != nil {
			log.Error("Error marshalling JSON data for service bus:", err)
			return
		}
		msg := &azservicebus.Message{
			Body:                  payload,
			SessionID:             &sessionId,
			ApplicationProperties: make(map[string]interface{})}

		msg.ApplicationProperties["Address"] = event.Address
		msg.ApplicationProperties["Identifier"] = event.Identifier

		msg.ApplicationProperties["Hash"] = event.TxHash
		msg.ApplicationProperties["OriginalTxHash"] = event.OriginalTxHash

		if identifier == core.BuiltInFunctionMultiESDTNFTTransfer {
			msg.ApplicationProperties["isNFT"] = isNFT
		}
		messages = append(messages, msg)
	}
	err := sb.publishFanout(sb.cfg.EventsExchange, messages)
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
	messages := make([]*azservicebus.Message, 0)

	msg := &azservicebus.Message{
		Body:                  revertBlockBytes,
		SessionID:             &revertBlock.Hash,
		ApplicationProperties: make(map[string]interface{})}

	msg.ApplicationProperties["Hash"] = revertBlock.Hash
	messages = append(messages, msg)

	err = sb.publishFanout(sb.cfg.RevertEventsExchange, messages)
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
	messages := make([]*azservicebus.Message, 0)

	msg := &azservicebus.Message{
		Body:                  finalizedBlockBytes,
		SessionID:             &finalizedBlock.Hash,
		ApplicationProperties: make(map[string]interface{})}

	msg.ApplicationProperties["Hash"] = finalizedBlock.Hash
	messages = append(messages, msg)

	err = sb.publishFanout(sb.cfg.FinalizedEventsExchange, messages)
	if err != nil {
		log.Error("failed to publish finalized event to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishTxsToExchange(blockTxs data.BlockTxs) {
	messages := make([]*azservicebus.Message, 0)

	for _, tx := range blockTxs.Txs {
		event, err := sb.marshaller.Marshal(tx)
		if err != nil {
			log.Error("could not marshal block scrs event", "err", err.Error())
			return
		}
		msg := &azservicebus.Message{
			Body:                  event,
			SessionID:             &blockTxs.Hash,
			ApplicationProperties: make(map[string]interface{})}

		msg.ApplicationProperties["Hash"] = blockTxs.Hash
		messages = append(messages, msg)
	}

	err := sb.publishFanout(sb.cfg.BlockTxsExchange, messages)
	if err != nil {
		log.Error("failed to publish block txs event to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishAlteredAccounts(accounts data.AlteredAccountsEvent) {
	messages := make([]*azservicebus.Message, 0)

	for _, account := range accounts.Accounts {
		event, err := sb.marshaller.Marshal(account)
		if err != nil {
			log.Error("could not marshal altered accounts event", "err", err.Error())
			return
		}
		msg := &azservicebus.Message{
			Body:                  event,
			SessionID:             &account.Address,
			ApplicationProperties: make(map[string]interface{})}

		msg.ApplicationProperties["Address"] = account.Address
		msg.ApplicationProperties["Hash"] = accounts.Hash
		messages = append(messages, msg)
	}
	err := sb.publishFanout(sb.cfg.AlteredAccountsExchange, messages)
	if err != nil {
		log.Error("failed to publish altered accounts event to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishScrsToExchange(blockScrs data.BlockScrs) {
	messages := make([]*azservicebus.Message, 0)

	for _, scr := range blockScrs.Scrs {
		event, err := sb.marshaller.Marshal(scr)
		if err != nil {
			log.Error("could not marshal block scrs event", "err", err.Error())
			return
		}
		msg := &azservicebus.Message{
			Body:                  event,
			SessionID:             &blockScrs.Hash,
			ApplicationProperties: make(map[string]interface{})}

		msg.ApplicationProperties["BlockHash"] = blockScrs.Hash
		messages = append(messages, msg)
	}

	err := sb.publishFanout(sb.cfg.BlockScrsExchange, messages)
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

	messages := make([]*azservicebus.Message, 0)

	msg := &azservicebus.Message{
		Body:                  txsBlockBytes,
		SessionID:             &blockTxs.Hash,
		ApplicationProperties: make(map[string]interface{})}

	msg.ApplicationProperties["Hash"] = blockTxs.Hash
	messages = append(messages, msg)

	err = sb.publishFanout(sb.cfg.BlockEventsExchange, messages)
	if err != nil {
		log.Error("failed to publish full block events to servicebus", "err", err.Error())
	}
}

func (sb *serviceBusPublisher) publishFanout(exchangeConfig config.ServiceBusExchangeConfig, payload []*azservicebus.Message) error {
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
