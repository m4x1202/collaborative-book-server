package cmd

/*import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	cb "github.com/m4x1202/collaborative-book"
	cbdynamodb "github.com/m4x1202/collaborative-book/internal/app/dynamodb"
	"github.com/m4x1202/collaborative-book/internal/app/utils"
)

func Test_Disconnect(t *testing.T) {
	player := cb.PlayerItem{
		PlayerInfo: &cb.PlayerInfo{},
	}
	payload, _ := attributevalue.MarshalMap(player)
	service := cbdynamodb.DBService{
		DB: &utils.FakeDynamoDB{
			Payload: []map[string]*types.AttributeValue{payload},
		},
	}
	request := new(events.APIGatewayWebsocketProxyRequest)
	request.RequestContext.ConnectionID = "a"
	if err := Disconnect(service, *request); err != nil {
		t.FailNow()
	}
}
*/
