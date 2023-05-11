package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	cb "github.com/m4x1202/collaborative-book"
	log "github.com/sirupsen/logrus"
)

const (
	DynamoDBTable = "collaborative-book-connections"
)

// Ensure DBService implements cb.DBService
var _ cb.DBService = (*DBService)(nil)

// A service that holds dynamodb db service functionality
type DBService struct {
	ctx    context.Context
	client DynamoDBAPI
}

type DynamoDBAPI interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

func NewDBService(ctx context.Context, conf aws.Config) DBService {
	return DBService{
		ctx:    ctx,
		client: dynamodb.NewFromConfig(conf),
	}
}

func (dbs DBService) UpdatePlayerItem(player *cb.PlayerItem) error {
	player.ExpirationTime = time.Now().Add(time.Hour * 2).Unix()
	marshaledPlayer, err := attributevalue.MarshalMap(player)
	if err != nil {
		return err
	}
	var updateExpression expression.UpdateBuilder
	for name, value := range marshaledPlayer {
		switch name {
		case "room", "connection_id":
			continue
		default:
			updateExpression = updateExpression.Set(
				expression.Name(name),
				expression.Value(value),
			)
		}
	}
	expr, err := expression.NewBuilder().
		WithUpdate(updateExpression).
		Build()
	if err != nil {
		return err
	}

	updateItemInput := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(DynamoDBTable),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Key:                       marshalPlayerKey(*player),
		UpdateExpression:          expr.Update(),
	}

	if _, err = dbs.client.UpdateItem(dbs.ctx, updateItemInput); err != nil {
		return err
	}
	return nil
}

func (dbs DBService) ResetPlayerItem(player *cb.PlayerItem) error {
	playerInfo := cb.PlayerInfo{
		UserName:   player.PlayerInfo.UserName,
		Status:     cb.Waiting,
		Spectating: true,
	}
	player.PlayerInfo = &playerInfo

	return dbs.UpdatePlayerItem(player)
}

func (dbs DBService) RemovePlayerItem(player cb.PlayerItem) error {
	deleteItemInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(DynamoDBTable),
		Key:       marshalPlayerKey(player),
	}
	if _, err := dbs.client.DeleteItem(dbs.ctx, deleteItemInput); err != nil {
		return err
	}
	log.Debugf("Player with connection_id %s removed from DynamoDB", player.ConnectionID)
	return nil
}

func (dbs DBService) RemoveConnection(connectionID string) error {
	log.Infof("RemoveConnection triggered for connection_id %s", connectionID)
	var conditionExpression expression.ConditionBuilder
	conditionExpression = expression.Name("connection_id").Equal(expression.Value(connectionID))

	expr, err := expression.NewBuilder().
		WithFilter(conditionExpression).
		Build()
	if err != nil {
		return err
	}

	scanInput := &dynamodb.ScanInput{
		TableName:                 aws.String(DynamoDBTable),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
	}

	scanOutput, err := dbs.client.Scan(dbs.ctx, scanInput)
	if err != nil {
		return err
	}
	log.Tracef("Scan output: %v", *scanOutput)

	var players []cb.PlayerItem
	err = attributevalue.UnmarshalListOfMaps(scanOutput.Items, &players)
	if err != nil {
		return err
	}
	var errs []error
	for _, player := range players {
		log.Debugf("Player with connection_id %s is in room %s", connectionID, player.Room)
		if player.PlayerInfo.IsAdmin {
			continue
		}
		if player.PlayerInfo.RoomState != cb.Lobby {
			continue
		}

		err = dbs.RemovePlayerItem(player)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("Removing connection %s resulted in errors: %v", connectionID, errs)
	}
	return nil
}

func (dbs DBService) GetPlayerItems(room string) (cb.PlayerItemList, error) {
	var keyConditionExpression expression.KeyConditionBuilder
	keyConditionExpression = expression.Key("room").Equal(expression.Value(room))

	expr, err := expression.NewBuilder().
		WithKeyCondition(keyConditionExpression).
		Build()
	if err != nil {
		return nil, err
	}

	queryInput := &dynamodb.QueryInput{
		TableName:                 aws.String(DynamoDBTable),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
	}

	queryOutput, err := dbs.client.Query(dbs.ctx, queryInput)
	if err != nil {
		return nil, err
	}
	log.Tracef("Query output: %v", *queryOutput)

	var players cb.PlayerItemList
	for _, i := range queryOutput.Items {
		player := &cb.PlayerItem{}
		if err := attributevalue.UnmarshalMap(i, player); err != nil {
			log.Error(err)
			continue
		}
		players = append(players, player)
	}
	return players, nil
}

func marshalPlayerKey(player cb.PlayerItem) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"room":          &types.AttributeValueMemberS{Value: player.Room},
		"connection_id": &types.AttributeValueMemberS{Value: player.ConnectionID},
	}
}
