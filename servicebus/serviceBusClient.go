package servicebus

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/truststaking/mx-chain-notifier-go/config"
	"github.com/truststaking/mx-chain-notifier-go/data"
)

const (
	reconnectRetryMs = 500
)

type serviceBusClient struct {
	url    string
	pubMut sync.Mutex

	client *azservicebus.Client
}

// NewserviceBusClient creates a new rabbitMQ client instance
func NewServiceBusClient(url string) (*serviceBusClient, error) {
	sb := &serviceBusClient{
		url:    url,
		pubMut: sync.Mutex{},
	}

	err := sb.connect()
	if err != nil {
		return nil, err
	}

	return sb, nil
}

// Publish will publish an item on the servicebus channel
func (sb *serviceBusClient) Publish(exchangeConfig config.ServiceBusExchangeConfig, cfg config.AzureServiceBusConfig, payload []byte) error {
	if !exchangeConfig.Enabled {
		return nil
	}
	sb.pubMut.Lock()
	defer sb.pubMut.Unlock()

	sender, err := sb.client.NewSender(exchangeConfig.Topic, nil)
	if err != nil {
		log.Error("could not send the payload to azure service bus", "err", err.Error())
		return err
	}

	var events data.BlockEventsWithOrder

	err = json.Unmarshal(payload, &events)
	if err != nil {
		log.Error("Error unmarshalling JSON data for service bus:", err)
		return err
	}

	currentMessageBatch, err := sender.NewMessageBatch(context.TODO(), nil)
	if err != nil {
		log.Error("error creating message batch for service bus:", err)
		return err
	}

	for i := 0; i < len(events.Events); i++ {
		identifier := events.Events[i].Identifier
		sessionId := events.Events[i].Address
		isNFT := "true"

		if identifier == core.BuiltInFunctionESDTNFTCreate ||
			identifier == core.BuiltInFunctionESDTNFTBurn ||
			identifier == core.BuiltInFunctionESDTNFTUpdateAttributes ||
			identifier == core.BuiltInFunctionESDTNFTAddURI ||
			identifier == core.BuiltInFunctionESDTNFTAddQuantity ||
			identifier == core.BuiltInFunctionMultiESDTNFTTransfer ||
			identifier == core.BuiltInFunctionESDTNFTTransfer ||
			identifier == core.BuiltInFunctionESDTTransfer {
			hexStr := hex.EncodeToString(events.Events[i].Topics[1])
			if hexStr == "" {
				isNFT = "false"
			}
			sessionId = string(events.Events[i].Topics[0])
		}
		event, err := json.Marshal(events.Events[i])
		if err != nil {
			log.Error("Error marshalling JSON data for service bus:", err)
			return err
		}
		msg := &azservicebus.Message{
			Body:                  event,
			SessionID:             &sessionId,
			ApplicationProperties: make(map[string]interface{})}

		msg.ApplicationProperties["Address"] = events.Events[i].Address
		msg.ApplicationProperties["Identifier"] = events.Events[i].Identifier

		if identifier == core.BuiltInFunctionMultiESDTNFTTransfer {
			msg.ApplicationProperties["isNFT"] = isNFT
		}
		err = currentMessageBatch.AddMessage(msg, nil)

		if errors.Is(err, azservicebus.ErrMessageTooLarge) {
			if currentMessageBatch.NumMessages() == 0 {
				log.Error("Single message is too large to be sent in a batch.")
				return err
			}

			log.Info("Message batch is full. Sending it and creating a new one.")

			// send what we have since the batch is full
			err := sender.SendMessageBatch(context.TODO(), currentMessageBatch, nil)

			if err != nil {
				// TODO: handle loop retry
				log.Error("Error sending the batch of messages", err)
				return err
			}

			// Create a new batch and retry adding this message to our batch.
			newBatch, err := sender.NewMessageBatch(context.TODO(), nil)

			if err != nil {
				log.Error("Error creating a new batch of messages", err)
				return err
			}

			currentMessageBatch = newBatch

			// rewind the counter and attempt to add the message again (this batch
			// was full so it didn't go out with the previous SendMessageBatch call).
			i--
		} else if err != nil {
			log.Error("Error adding message to batch", currentMessageBatch.NumMessages(), err.Error())
			return err
		}
	}

	// check if any messages are remaining to be sent.
	if currentMessageBatch.NumMessages() > 0 {
		err := sender.SendMessageBatch(context.TODO(), currentMessageBatch, nil)
		if err != nil {
			log.Error("Error send remaining messages in batch", err.Error())
			return err
		}
	}

	sender.Close(context.TODO())
	return nil
}

func (sb *serviceBusClient) connect() error {
	client, err := azservicebus.NewClientFromConnectionString(sb.url, nil)
	if err != nil {
		return err
	}
	sb.client = client
	return nil
}

// Reconnect will try to reconnect to rabbitmq
func (sb *serviceBusClient) Reconnect() {
	for {
		time.Sleep(time.Millisecond * reconnectRetryMs)

		err := sb.connect()
		if err != nil {
			log.Debug("could not reconnect", "err", err.Error())
		} else {
			log.Debug("connection established after reconnect attempts")
			break
		}
	}
}

// Close will close rabbitMq client connection
func (sb *serviceBusClient) Close() {
	err := sb.client.Close(context.TODO())
	if err != nil {
		log.Error("failed to close servicebus client", "err", err.Error())
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (sb *serviceBusClient) IsInterfaceNil() bool {
	return sb == nil
}

const METACHAIN_SHARD_ID = 4294967295

func isAddressOfMetachain(pubKey []byte) bool {
	metachainPrefix := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	pubKeyPrefix := pubKey[:len(metachainPrefix)]
	if bytes.Equal(pubKeyPrefix, metachainPrefix) {
		return true
	}
	zeroAddress := make([]byte, 32)
	return bytes.Equal(pubKey, zeroAddress)
}

func getShardOfAddress(hexPubKey string) int {
	numShards := 3
	maskHigh := 0b11
	maskLow := 0b01

	pubKey, _ := hex.DecodeString(hexPubKey)
	lastByteOfPubKey := pubKey[31]

	if isAddressOfMetachain(pubKey) {
		return METACHAIN_SHARD_ID
	}

	shard := int(lastByteOfPubKey) & maskHigh
	if shard > numShards-1 {
		shard = int(lastByteOfPubKey) & maskLow
	}
	return shard
}
