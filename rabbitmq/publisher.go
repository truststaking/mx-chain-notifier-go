package rabbitmq

import (
	"context"

	"github.com/multiversx/mx-chain-core-go/core/check"
	"github.com/multiversx/mx-chain-core-go/data/alteredAccount"
	"github.com/multiversx/mx-chain-core-go/marshal"
	logger "github.com/multiversx/mx-chain-logger-go"
	"github.com/streadway/amqp"
	"github.com/truststaking/mx-chain-notifier-go/common"
	"github.com/truststaking/mx-chain-notifier-go/config"
	"github.com/truststaking/mx-chain-notifier-go/data"
)

const (
	emptyStr = ""
)

var log = logger.GetOrCreate("rabbitmq")

// ArgsRabbitMqPublisher defines the arguments needed for rabbitmq publisher creation
type ArgsRabbitMqPublisher struct {
	Client     RabbitMqClient
	Config     config.RabbitMQConfig
	Marshaller marshal.Marshalizer
}

type rabbitMqPublisher struct {
	client     RabbitMqClient
	marshaller marshal.Marshalizer
	cfg        config.RabbitMQConfig

	broadcast                     chan data.BlockEvents
	broadcastRevert               chan data.RevertBlock
	broadcastFinalized            chan data.FinalizedBlock
	broadcastTxs                  chan data.BlockTxs
	broadcastBlockEventsWithOrder chan data.BlockEventsWithOrder
	alteredAccounts				  chan *alteredAccount.AlteredAccount
	broadcastScrs                 chan data.BlockScrs

	cancelFunc func()
	closeChan  chan struct{}
}

// NewRabbitMqPublisher creates a new rabbitMQ publisher instance
func NewRabbitMqPublisher(args ArgsRabbitMqPublisher) (*rabbitMqPublisher, error) {
	err := checkArgs(args)
	if err != nil {
		return nil, err
	}

	rp := &rabbitMqPublisher{
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

	err = rp.createExchanges()
	if err != nil {
		return nil, err
	}

	return rp, nil
}

func checkArgs(args ArgsRabbitMqPublisher) error {
	if check.IfNil(args.Client) {
		return ErrNilRabbitMqClient
	}
	if check.IfNil(args.Marshaller) {
		return common.ErrNilMarshaller
	}

	if args.Config.EventsExchange.Name == "" {
		return ErrInvalidRabbitMqExchangeName
	}
	if args.Config.EventsExchange.Type == "" {
		return ErrInvalidRabbitMqExchangeType
	}
	if args.Config.RevertEventsExchange.Name == "" {
		return ErrInvalidRabbitMqExchangeName
	}
	if args.Config.RevertEventsExchange.Type == "" {
		return ErrInvalidRabbitMqExchangeType
	}
	if args.Config.FinalizedEventsExchange.Name == "" {
		return ErrInvalidRabbitMqExchangeName
	}
	if args.Config.FinalizedEventsExchange.Type == "" {
		return ErrInvalidRabbitMqExchangeType
	}
	if args.Config.BlockTxsExchange.Name == "" {
		return ErrInvalidRabbitMqExchangeName
	}
	if args.Config.BlockTxsExchange.Type == "" {
		return ErrInvalidRabbitMqExchangeType
	}
	if args.Config.BlockScrsExchange.Name == "" {
		return ErrInvalidRabbitMqExchangeName
	}
	if args.Config.BlockScrsExchange.Type == "" {
		return ErrInvalidRabbitMqExchangeType
	}
	if args.Config.BlockEventsExchange.Name == "" {
		return ErrInvalidRabbitMqExchangeName
	}
	if args.Config.BlockEventsExchange.Type == "" {
		return ErrInvalidRabbitMqExchangeType
	}

	return nil
}

// checkAndCreateExchanges creates exchanges if they are not existing already
func (rp *rabbitMqPublisher) createExchanges() error {
	err := rp.createExchange(rp.cfg.EventsExchange)
	if err != nil {
		return err
	}
	err = rp.createExchange(rp.cfg.RevertEventsExchange)
	if err != nil {
		return err
	}
	err = rp.createExchange(rp.cfg.FinalizedEventsExchange)
	if err != nil {
		return err
	}
	err = rp.createExchange(rp.cfg.BlockTxsExchange)
	if err != nil {
		return err
	}
	err = rp.createExchange(rp.cfg.BlockScrsExchange)
	if err != nil {
		return err
	}
	err = rp.createExchange(rp.cfg.BlockEventsExchange)
	if err != nil {
		return err
	}

	return nil
}

func (rp *rabbitMqPublisher) createExchange(conf config.RabbitMQExchangeConfig) error {
	err := rp.client.ExchangeDeclare(conf.Name, conf.Type)
	if err != nil {
		return err
	}

	log.Info("checked and declared rabbitMQ exchange", "name", conf.Name, "type", conf.Type)

	return nil
}

// Run is launched as a goroutine and listens for events on the exposed channels
func (rp *rabbitMqPublisher) Run() {
	var ctx context.Context
	ctx, rp.cancelFunc = context.WithCancel(context.Background())

	go rp.run(ctx)
}

func (rp *rabbitMqPublisher) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debug("RabbitMQ publisher is stopping...")
			rp.client.Close()
		case events := <-rp.broadcast:
			rp.publishToExchanges(events)
		case revertBlock := <-rp.broadcastRevert:
			rp.publishRevertToExchange(revertBlock)
		case finalizedBlock := <-rp.broadcastFinalized:
			rp.publishFinalizedToExchange(finalizedBlock)
		case blockTxs := <-rp.broadcastTxs:
			rp.publishTxsToExchange(blockTxs)
		case blockScrs := <-rp.broadcastScrs:
			rp.publishScrsToExchange(blockScrs)
		case blockEvents := <-rp.broadcastBlockEventsWithOrder:
			rp.publishBlockEventsWithOrderToExchange(blockEvents)
		case alteredAccounts := <-rp.alteredAccounts:
			rp.publishAlteredAccounts(alteredAccounts)
		case err := <-rp.client.ConnErrChan():
			if err != nil {
				log.Error("rabbitMQ connection failure", "err", err.Error())
				rp.client.Reconnect()
			}
		case err := <-rp.client.CloseErrChan():
			if err != nil {
				log.Error("rabbitMQ channel failure", "err", err.Error())
				rp.client.ReopenChannel()
			}
		}
	}
}

// Broadcast will handle the block events pushed by producers and sends them to rabbitMQ channel
func (rp *rabbitMqPublisher) Broadcast(events data.BlockEvents) {
	select {
	case rp.broadcast <- events:
	case <-rp.closeChan:
	}
}

// BroadcastRevert will handle the revert event pushed by producers and sends them to rabbitMQ channel
func (rp *rabbitMqPublisher) BroadcastRevert(events data.RevertBlock) {
	select {
	case rp.broadcastRevert <- events:
	case <-rp.closeChan:
	}
}

// BroadcastFinalized will handle the finalized event pushed by producers and sends them to rabbitMQ channel
func (rp *rabbitMqPublisher) BroadcastFinalized(events data.FinalizedBlock) {
	select {
	case rp.broadcastFinalized <- events:
	case <-rp.closeChan:
	}
}

// BroadcastTxs will handle the txs event pushed by producers and sends them to rabbitMQ channel
func (rp *rabbitMqPublisher) BroadcastTxs(events data.BlockTxs) {
	select {
	case rp.broadcastTxs <- events:
	case <-rp.closeChan:
	}
}

// BroadcastScrs will handle the scrs event pushed by producers and sends them to rabbitMQ channel
func (rp *rabbitMqPublisher) BroadcastScrs(events data.BlockScrs) {
	select {
	case rp.broadcastScrs <- events:
	case <-rp.closeChan:
	}
}

func (rp *rabbitMqPublisher) BroadcastAlteredAccounts(accounts *alteredAccount.AlteredAccount) {
	select {
	case rp.alteredAccounts <- accounts:
	case <-rp.closeChan:
	}
}

// BroadcastBlockEventsWithOrder will handle the full block events pushed by producers and sends them to rabbitMQ channel
func (rp *rabbitMqPublisher) BroadcastBlockEventsWithOrder(events data.BlockEventsWithOrder) {
	select {
	case rp.broadcastBlockEventsWithOrder <- events:
	case <-rp.closeChan:
	}
}

func (rp *rabbitMqPublisher) publishToExchanges(events data.BlockEvents) {
	eventsBytes, err := rp.marshaller.Marshal(events)
	if err != nil {
		log.Error("could not marshal events", "err", err.Error())
		return
	}

	err = rp.publishFanout(rp.cfg.EventsExchange.Name, eventsBytes)
	if err != nil {
		log.Error("failed to publish events to rabbitMQ", "err", err.Error())
	}
}

func (rp *rabbitMqPublisher) publishRevertToExchange(revertBlock data.RevertBlock) {
	revertBlockBytes, err := rp.marshaller.Marshal(revertBlock)
	if err != nil {
		log.Error("could not marshal revert event", "err", err.Error())
		return
	}

	err = rp.publishFanout(rp.cfg.RevertEventsExchange.Name, revertBlockBytes)
	if err != nil {
		log.Error("failed to publish revert event to rabbitMQ", "err", err.Error())
	}
}

func (rp *rabbitMqPublisher) publishFinalizedToExchange(finalizedBlock data.FinalizedBlock) {
	finalizedBlockBytes, err := rp.marshaller.Marshal(finalizedBlock)
	if err != nil {
		log.Error("could not marshal finalized event", "err", err.Error())
		return
	}

	err = rp.publishFanout(rp.cfg.FinalizedEventsExchange.Name, finalizedBlockBytes)
	if err != nil {
		log.Error("failed to publish finalized event to rabbitMQ", "err", err.Error())
	}
}

func (rp *rabbitMqPublisher) publishTxsToExchange(blockTxs data.BlockTxs) {
	txsBlockBytes, err := rp.marshaller.Marshal(blockTxs)
	if err != nil {
		log.Error("could not marshal block txs event", "err", err.Error())
		return
	}

	err = rp.publishFanout(rp.cfg.BlockTxsExchange.Name, txsBlockBytes)
	if err != nil {
		log.Error("failed to publish block txs event to rabbitMQ", "err", err.Error())
	}
}

func (rp *rabbitMqPublisher) publishScrsToExchange(blockScrs data.BlockScrs) {
	scrsBlockBytes, err := rp.marshaller.Marshal(blockScrs)
	if err != nil {
		log.Error("could not marshal block scrs event", "err", err.Error())
		return
	}

	err = rp.publishFanout(rp.cfg.BlockScrsExchange.Name, scrsBlockBytes)
	if err != nil {
		log.Error("failed to publish block scrs event to rabbitMQ", "err", err.Error())
	}
}

func (rp *rabbitMqPublisher) publishBlockEventsWithOrderToExchange(blockTxs data.BlockEventsWithOrder) {
	txsBlockBytes, err := rp.marshaller.Marshal(blockTxs)
	if err != nil {
		log.Error("could not marshal block txs event", "err", err.Error())
		return
	}

	err = rp.publishFanout(rp.cfg.BlockEventsExchange.Name, txsBlockBytes)
	if err != nil {
		log.Error("failed to publish full block events to rabbitMQ", "err", err.Error())
	}
}


func (rp *rabbitMqPublisher) publishAlteredAccounts(accounts *alteredAccount.AlteredAccount) {
	alteredAccountBytes, err := rp.marshaller.Marshal(accounts)
	if err != nil {
		log.Error("could not marshal altered accounts event", "err", err.Error())
		return
	}

	err = rp.publishFanout(rp.cfg.BlockEventsExchange.Name, alteredAccountBytes)
	if err != nil {
		log.Error("failed to publish altered accounts to rabbitMQ", "err", err.Error())
	}
}

func (rp *rabbitMqPublisher) publishFanout(exchangeName string, payload []byte) error {
	return rp.client.Publish(
		exchangeName,
		emptyStr,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			Body: payload,
		},
	)
}

// Close will close the channels
func (rp *rabbitMqPublisher) Close() error {
	if rp.cancelFunc != nil {
		rp.cancelFunc()
	}

	close(rp.closeChan)

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (rp *rabbitMqPublisher) IsInterfaceNil() bool {
	return rp == nil
}
