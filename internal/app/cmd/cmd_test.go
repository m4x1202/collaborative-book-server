package cmd

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	cb "github.com/m4x1202/collaborative-book"
	mocks "github.com/m4x1202/collaborative-book/mocks"
	"github.com/stretchr/testify/mock"
)

func Test_Disconnect(t *testing.T) {
	service := &mocks.DBService{}
	service.On("RemoveConnection", mock.AnythingOfType("string")).Return(nil)
	if err := Disconnect(service, events.APIGatewayWebsocketProxyRequest{}); err != nil {
		t.FailNow()
	}
}

func Test_sendRoomUpdate(t *testing.T) {
	service := &mocks.WSService{}
	service.On("PostToConnection", mock.AnythingOfType("[]string"), mock.AnythingOfType("interface{}")).Return(nil)
	var playerList cb.PlayerItemList
	if err := sendRoomUpdate(service, playerList); err != nil {
		t.FailNow()
	}
}
