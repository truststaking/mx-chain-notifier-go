package factory

import (
	"github.com/multiversx/mx-chain-core-go/marshal"
	"github.com/truststaking/mx-chain-notifier-go/common"
	"github.com/truststaking/mx-chain-notifier-go/config"
	"github.com/truststaking/mx-chain-notifier-go/disabled"
	"github.com/truststaking/mx-chain-notifier-go/rabbitmq"
	"github.com/truststaking/mx-chain-notifier-go/servicebus"
)

// CreatePublisher creates publisher component
func CreatePublisher(apiType string, config config.MainConfig, marshaller marshal.Marshalizer) (rabbitmq.PublisherService, error) {
	switch apiType {
	case common.MessageQueuePublisherType:
		return createRabbitMqPublisher(config.RabbitMQ, marshaller)
	case common.ServiceBusQueuePublisherType:
		return createServiceBusPublisher(config.AzureServiceBus, marshaller)
	case common.WSPublisherType:
		return &disabled.Publisher{}, nil
	default:
		return nil, common.ErrInvalidAPIType
	}
}

func createRabbitMqPublisher(config config.RabbitMQConfig, marshaller marshal.Marshalizer) (rabbitmq.PublisherService, error) {
	rabbitClient, err := rabbitmq.NewRabbitMQClient(config.Url)
	if err != nil {
		return nil, err
	}

	rabbitMqPublisherArgs := rabbitmq.ArgsRabbitMqPublisher{
		Client:     rabbitClient,
		Config:     config,
		Marshaller: marshaller,
	}
	rabbitPublisher, err := rabbitmq.NewRabbitMqPublisher(rabbitMqPublisherArgs)
	if err != nil {
		return nil, err
	}

	return rabbitPublisher, nil
}

func createServiceBusPublisher(config config.AzureServiceBusConfig, marshaller marshal.Marshalizer) (servicebus.PublisherService, error) {
	serviceSubClient, err := servicebus.NewServiceBusClient(config.ServiceBusConnectionString)
	if err != nil {
		return nil, err
	}

	serviceSubPublisherArgs := servicebus.ArgsServiceBusPublisher{
		Client:     serviceSubClient,
		Config:     config,
		Marshaller: marshaller,
	}
	serviceSubPublisher, err := servicebus.NewServiceBusPublisher(serviceSubPublisherArgs)
	if err != nil {
		return nil, err
	}

	return serviceSubPublisher, nil
}
