package ws

import (
	"net/http"

	"github.com/multiversx/mx-chain-core-go/core/check"
	"github.com/multiversx/mx-chain-core-go/marshal"
	"github.com/truststaking/mx-chain-notifier-go/common"
	"github.com/truststaking/mx-chain-notifier-go/dispatcher"
)

// ArgsWebSocketProcessor defines the argument needed to create a websocketHandler
type ArgsWebSocketProcessor struct {
	Hub        dispatcher.Hub
	Upgrader   dispatcher.WSUpgrader
	Marshaller marshal.Marshalizer
}

type websocketProcessor struct {
	hub        dispatcher.Hub
	upgrader   dispatcher.WSUpgrader
	marshaller marshal.Marshalizer
}

// NewWebSocketProcessor creates a new websocketProcessor component
func NewWebSocketProcessor(args ArgsWebSocketProcessor) (*websocketProcessor, error) {
	err := checkArgs(args)
	if err != nil {
		return nil, err
	}

	return &websocketProcessor{
		hub:        args.Hub,
		upgrader:   args.Upgrader,
		marshaller: args.Marshaller,
	}, nil
}

func checkArgs(args ArgsWebSocketProcessor) error {
	if check.IfNil(args.Hub) {
		return ErrNilHubHandler
	}
	if args.Upgrader == nil {
		return ErrNilWSUpgrader
	}
	if check.IfNil(args.Marshaller) {
		return common.ErrNilMarshaller
	}

	return nil
}

// ServeHTTP is the entry point used by a http server to serve the websocket upgrader
func (wh *websocketProcessor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := wh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("failed upgrading connection", "err", err.Error())
		return
	}

	args := argsWebSocketDispatcher{
		Hub:        wh.hub,
		Conn:       conn,
		Marshaller: wh.marshaller,
	}
	wsDispatcher, err := newWebSocketDispatcher(args)
	if err != nil {
		log.Error("failed creating a new websocket dispatcher", "err", err.Error())
		return
	}
	wsDispatcher.hub.RegisterEvent(wsDispatcher)

	go wsDispatcher.writePump()
	go wsDispatcher.readPump()
}

// IsInterfaceNil returns true if there is no value under the interface
func (wh *websocketProcessor) IsInterfaceNil() bool {
	return wh == nil
}
