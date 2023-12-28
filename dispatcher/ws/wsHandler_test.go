package ws_test

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/core/check"
	"github.com/multiversx/mx-chain-core-go/core/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truststaking/mx-chain-notifier-go/common"
	"github.com/truststaking/mx-chain-notifier-go/dispatcher/ws"
	"github.com/truststaking/mx-chain-notifier-go/mocks"
)

func createMockArgsWSHandler() ws.ArgsWebSocketProcessor {
	return ws.ArgsWebSocketProcessor{
		Hub:        &mocks.HubStub{},
		Upgrader:   &mocks.WSUpgraderStub{},
		Marshaller: &mock.MarshalizerMock{},
	}
}

func TestNewWebSocketHandler(t *testing.T) {
	t.Parallel()

	t.Run("nil hub handler", func(t *testing.T) {
		t.Parallel()

		args := createMockArgsWSHandler()
		args.Hub = nil

		wh, err := ws.NewWebSocketProcessor(args)
		require.True(t, check.IfNil(wh))
		assert.Equal(t, ws.ErrNilHubHandler, err)
	})

	t.Run("nil ws upgrader", func(t *testing.T) {
		t.Parallel()

		args := createMockArgsWSHandler()
		args.Upgrader = nil

		wh, err := ws.NewWebSocketProcessor(args)
		require.True(t, check.IfNil(wh))
		assert.Equal(t, ws.ErrNilWSUpgrader, err)
	})

	t.Run("nil marshaller", func(t *testing.T) {
		t.Parallel()

		args := createMockArgsWSHandler()
		args.Marshaller = nil

		wh, err := ws.NewWebSocketProcessor(args)
		require.True(t, check.IfNil(wh))
		assert.Equal(t, common.ErrNilMarshaller, err)
	})

	t.Run("should work", func(t *testing.T) {
		t.Parallel()

		args := createMockArgsWSHandler()
		wh, err := ws.NewWebSocketProcessor(args)
		require.False(t, check.IfNil(wh))
		require.Nil(t, err)
	})
}
