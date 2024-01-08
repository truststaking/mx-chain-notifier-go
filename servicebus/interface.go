package servicebus

import (
	"github.com/multiversx/mx-chain-core-go/data/alteredAccount"
	"github.com/truststaking/mx-chain-notifier-go/config"
	"github.com/truststaking/mx-chain-notifier-go/data"
)

// ServiceBusClient defines the behaviour of a rabbitMq client
type ServiceBusClient interface {
	Publish(exchange config.ServiceBusExchangeConfig, cfg config.AzureServiceBusConfig, payload []byte) error
	Close()
	IsInterfaceNil() bool
}

// PublisherService defines the behaviour of a publisher component which should be
// able to publish received events and broadcast them to channels
type PublisherService interface {
	Run()
	BroadcastAlteredAccounts(accounts *alteredAccount.AlteredAccount)
	Broadcast(events data.BlockEvents)
	BroadcastRevert(event data.RevertBlock)
	BroadcastFinalized(event data.FinalizedBlock)
	BroadcastTxs(event data.BlockTxs)
	BroadcastScrs(event data.BlockScrs)
	BroadcastBlockEventsWithOrder(event data.BlockEventsWithOrder)
	Close() error
	IsInterfaceNil() bool
}
