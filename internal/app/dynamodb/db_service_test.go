package dynamodb

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	cb "github.com/m4x1202/collaborative-book"
)

type mockDynamoDBAPI struct {
	UpdateItemAPI func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItemAPI func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	ScanAPI       func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	QueryAPI      func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

func (m mockDynamoDBAPI) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return m.UpdateItemAPI(ctx, params, optFns...)
}
func (m mockDynamoDBAPI) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	return m.DeleteItemAPI(ctx, params, optFns...)
}
func (m mockDynamoDBAPI) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	return m.ScanAPI(ctx, params, optFns...)
}
func (m mockDynamoDBAPI) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	return m.QueryAPI(ctx, params, optFns...)
}

func Test_UpdatePlayerItem(t *testing.T) {
	service := DBService{
		client: func(t *testing.T) DynamoDBAPI {
			return &mockDynamoDBAPI{
				UpdateItemAPI: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					t.Helper()
					return &dynamodb.UpdateItemOutput{}, nil
				},
			}
		}(t),
	}
	if err := service.UpdatePlayerItem(&cb.PlayerItem{}); err != nil {
		t.FailNow()
	}
}

func Test_ResetPlayerItem(t *testing.T) {
	service := DBService{
		client: func(t *testing.T) DynamoDBAPI {
			return &mockDynamoDBAPI{
				UpdateItemAPI: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					t.Helper()
					return &dynamodb.UpdateItemOutput{}, nil
				},
			}
		}(t),
	}
	if err := service.ResetPlayerItem(&cb.PlayerItem{PlayerInfo: &cb.PlayerInfo{}}); err != nil {
		t.FailNow()
	}
}

func Test_RemovePlayerItem(t *testing.T) {
	service := DBService{
		client: func(t *testing.T) DynamoDBAPI {
			return &mockDynamoDBAPI{
				DeleteItemAPI: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
					t.Helper()
					return &dynamodb.DeleteItemOutput{}, nil
				},
			}
		}(t),
	}
	if err := service.RemovePlayerItem(cb.PlayerItem{}); err != nil {
		t.FailNow()
	}
}

func Test_RemoveConnection(t *testing.T) {
	player := cb.PlayerItem{
		PlayerInfo: &cb.PlayerInfo{},
	}
	payload, _ := attributevalue.MarshalMap(player)
	service := DBService{
		client: func(t *testing.T) DynamoDBAPI {
			return &mockDynamoDBAPI{
				ScanAPI: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
					t.Helper()
					return &dynamodb.ScanOutput{
						Items: []map[string]types.AttributeValue{payload},
					}, nil
				},
				DeleteItemAPI: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
					t.Helper()
					return &dynamodb.DeleteItemOutput{}, nil
				},
			}
		}(t),
	}
	if err := service.RemoveConnection("a"); err != nil {
		t.FailNow()
	}
}

func Test_GetPlayerItems(t *testing.T) {
	player := cb.PlayerItem{
		PlayerInfo: &cb.PlayerInfo{
			UserName: "a",
		},
	}
	payload, _ := attributevalue.MarshalMap(player)
	service := DBService{
		client: func(t *testing.T) DynamoDBAPI {
			return &mockDynamoDBAPI{
				QueryAPI: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
					t.Helper()
					return &dynamodb.QueryOutput{
						Items: []map[string]types.AttributeValue{payload},
					}, nil
				},
			}
		}(t),
	}
	players, err := service.GetPlayerItems("a")
	if err != nil {
		t.FailNow()
	}
	t.Log(players[0].PlayerInfo.UserName)
}
