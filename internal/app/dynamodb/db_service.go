package dynamodb

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	cb "github.com/m4x1202/collaborative-book"
	log "github.com/sirupsen/logrus"
)

const (
	DynamoDBTable = "collaborative-book-connections"
)

//Ensure DBService implements cb.DBService
var _ cb.DBService = (*DBService)(nil)

// A service that holds dynamodb db service functionality
type DBService struct {
	db *dynamodb.DynamoDB
}

func NewDBService(sess *session.Session) DBService {
	return DBService{
		db: dynamodb.New(sess),
	}
}

func (dbs DBService) UpdatePlayerItem(player cb.PlayerItem) error {
	marshaledPlayer, err := dynamodbattribute.MarshalMap(player)
	if err != nil {
		return err
	}
	var updateExpression expression.UpdateBuilder
	for name, value := range marshaledPlayer {
		if name == "room" || name == "connectionId" {
			continue
		}
		updateExpression = updateExpression.Set(
			expression.Name(name),
			expression.Value(value),
		)
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
		Key:                       marshalPlayerKey(player),
		UpdateExpression:          expr.Update(),
	}

	_, err = dbs.db.UpdateItem(updateItemInput)
	if err != nil {
		return err
	}
	return nil
}

func (dbs DBService) ResetPlayerItem(player *cb.PlayerItem) error {
	player.Contributions = nil
	player.IsAdmin = false
	player.Participants = nil
	player.LastStage = 0
	player.Spectating = true
	player.RoomState = cb.Lobby
	player.Status = cb.Waiting

	return dbs.UpdatePlayerItem(*player)
}

func (dbs DBService) RemovePlayerItem(player cb.PlayerItem) error {
	deleteItemInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(DynamoDBTable),
		Key:       marshalPlayerKey(player),
	}
	_, err := dbs.db.DeleteItem(deleteItemInput)
	if err != nil {
		return err
	}
	return nil
}

func (dbs DBService) RemoveConnection(connectionID string) error {
	scanInput := &dynamodb.ScanInput{
		TableName:            aws.String(DynamoDBTable),
		ProjectionExpression: aws.String("room"),
		FilterExpression:     aws.String("connectionId = :cid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":cid": {
				S: aws.String(connectionID),
			},
		},
	}

	scanOutput, err := dbs.db.Scan(scanInput)
	if err != nil {
		return err
	}
	log.Tracef("Scan output: %v", *scanOutput)
	if aws.Int64Value(scanOutput.Count) > 1 {
		return fmt.Errorf("More than one player with this connectionId")
	}

	var room string
	err = dynamodbattribute.Unmarshal(scanOutput.Items[0]["room"], &room)
	if err != nil {
		return err
	}
	log.Debugf("Player with connectionId %s is in room %s", connectionID, room)

	err = dbs.RemovePlayerItem(cb.PlayerItem{
		Room:         room,
		ConnectionID: connectionID,
	})
	if err != nil {
		return err
	}
	return nil
}

func (dbs DBService) GetPlayerItems(room string) (cb.PlayerItemList, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(DynamoDBTable),
		KeyConditionExpression: aws.String("room = :r"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":r": {
				S: aws.String(room),
			},
		},
	}

	queryOutput, err := dbs.db.Query(queryInput)
	if err != nil {
		return nil, err
	}
	log.Tracef("Query output: %v", *queryOutput)

	var players cb.PlayerItemList
	for _, i := range queryOutput.Items {
		player := &cb.PlayerItem{}
		if err := dynamodbattribute.UnmarshalMap(i, player); err != nil {
			log.Error(err)
			continue
		}
		players = append(players, player)
	}
	return players, nil
}

func marshalPlayerKey(player cb.PlayerItem) map[string]*dynamodb.AttributeValue {
	return map[string]*dynamodb.AttributeValue{
		"room": {
			S: aws.String(player.Room),
		},
		"connectionId": {
			S: aws.String(player.ConnectionID),
		},
	}
}
