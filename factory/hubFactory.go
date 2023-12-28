package factory

import (
	"github.com/truststaking/mx-chain-notifier-go/common"
	"github.com/truststaking/mx-chain-notifier-go/disabled"
	"github.com/truststaking/mx-chain-notifier-go/dispatcher"
	"github.com/truststaking/mx-chain-notifier-go/dispatcher/hub"
	"github.com/truststaking/mx-chain-notifier-go/filters"
)

// CreateHub creates a common hub component
func CreateHub(apiType string) (dispatcher.Hub, error) {
	switch apiType {
	case common.MessageQueuePublisherType:
		return &disabled.Hub{}, nil
	case common.ServiceBusQueuePublisherType:
		return &disabled.Hub{}, nil
	case common.WSPublisherType:
		return createHub()
	default:
		return nil, common.ErrInvalidAPIType
	}
}

func createHub() (dispatcher.Hub, error) {
	args := hub.ArgsCommonHub{
		Filter:             filters.NewDefaultFilter(),
		SubscriptionMapper: dispatcher.NewSubscriptionMapper(),
	}
	return hub.NewCommonHub(args)
}
