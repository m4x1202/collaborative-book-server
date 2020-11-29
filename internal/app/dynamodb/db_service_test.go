package dynamodb

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	cb "github.com/m4x1202/collaborative-book"
	"github.com/m4x1202/collaborative-book/internal/app/utils"
)

func Test_UpdatePlayerItem(t *testing.T) {
	service := DBService{
		DB: &utils.FakeDynamoDB{},
	}
	if err := service.UpdatePlayerItem(&cb.PlayerItem{}); err != nil {
		t.FailNow()
	}
}

func Test_ResetPlayerItem(t *testing.T) {
	service := DBService{
		DB: &utils.FakeDynamoDB{},
	}
	if err := service.ResetPlayerItem(&cb.PlayerItem{PlayerInfo: &cb.PlayerInfo{}}); err != nil {
		t.FailNow()
	}
}

func Test_RemovePlayerItem(t *testing.T) {
	service := DBService{
		DB: &utils.FakeDynamoDB{},
	}
	if err := service.RemovePlayerItem(cb.PlayerItem{}); err != nil {
		t.FailNow()
	}
}

func Test_RemoveConnection(t *testing.T) {
	player := cb.PlayerItem{
		PlayerInfo: &cb.PlayerInfo{},
	}
	payload, _ := dynamodbattribute.MarshalMap(player)
	service := DBService{
		DB: &utils.FakeDynamoDB{
			Payload: []map[string]*dynamodb.AttributeValue{payload},
		},
	}
	if err := service.RemoveConnection("a"); err != nil {
		t.FailNow()
	}
}

func Test_GetPlayerItems(t *testing.T) {
	player := cb.PlayerItem{
		PlayerInfo: &cb.PlayerInfo{},
	}
	payload, _ := dynamodbattribute.MarshalMap(player)
	service := DBService{
		DB: &utils.FakeDynamoDB{
			Payload: []map[string]*dynamodb.AttributeValue{payload},
		},
	}
	if _, err := service.GetPlayerItems("a"); err != nil {
		t.FailNow()
	}
}
