package rabbitmq

import (
	"github.com/streadway/amqp"
	"github.com/truststaking/mx-chain-notifier-go/data"
)

// RabbitMqClient defines the behaviour of a rabbitMq client
type RabbitMqClient interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	ExchangeDeclare(name, kind string) error
	ConnErrChan() chan *amqp.Error
	CloseErrChan() chan *amqp.Error
	Reconnect()
	ReopenChannel()
	Close()
	IsInterfaceNil() bool
}

// PublisherService defines the behaviour of a publisher component which should be
// able to publish received events and broadcast them to channels
type PublisherService interface {
	Run()
	BroadcastAlteredAccounts(accounts data.AlteredAccountsEvent)
	Broadcast(events data.BlockEvents)
	BroadcastRevert(event data.RevertBlock)
	BroadcastFinalized(event data.FinalizedBlock)
	BroadcastTxs(event data.BlockTxs)
	BroadcastScrs(event data.BlockScrs)
	BroadcastBlockEventsWithOrder(event data.BlockEventsWithOrder)
	Close() error
	IsInterfaceNil() bool
}
