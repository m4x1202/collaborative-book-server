package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/sirupsen/logrus"

	"github.com/gorilla/websocket"
)

const (
	AWSRegion          = "eu-central-1"
	APIGatewayEndpoint = "r8sc9tucc2.execute-api.eu-central-1.amazonaws.com/dev"
	DynamoDBTable      = "collaborative-book-connections"
	DefaultRoomName    = "unknown"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetReportCaller(true)
	log.SetLevel(log.TraceLevel)

	defer func() {
		if r := recover(); r != nil {
			log.Warnf("Recovered in main: %v", r)
		}
	}()

	lambda.Start(Handler)

	// serve static files
	/*client := gin.Default()
	  client.Static("/", "./client")
	  go client.Run(":8080")*/

	// websocket
	//server := gin.Default()
	//server.GET("/", func(c *gin.Context) {
	//	go handleWebsocket(c.Writer, c.Request)
	//})
	//server.Run(":8081")
}

/// BEGIN Server/Client Interface

type MessageType string

const (
	Registration MessageType = "registration"
	StartSession MessageType = "start_session"
	CloseRoom    MessageType = "close_room"
	SubmitStory  MessageType = "submit_story"
	ShowStory    MessageType = "show_story"
	RoomUmdate   MessageType = "room_update"
	RoundUpdate  MessageType = "round_update"
)

// Client

/**
 * Possible MessageTypes with according payload:
 * 	registration()
 * 	start_session(number of stages on start)
 * 	close_room()
 * 	submit_story(story text)
 * 	show_story(user name and stage to show the story from)
 */
type ClientMessage struct {
	MessageType MessageType `json:"type"` // registration, start_session, close_room, submit_story, show_story
	Room        string      `json:"room"`
	UserName    string      `json:"name"`
	Payload     string      `json:"payload"`
}

//Server

type ShowStoryPayload struct {
	UserName string `json:"user_name"`
	Stage    int    `json:"stage"`
}

type StartSessionPayload struct {
	LastStage int `json:"last_stage"`
}

type RegistrationResultString string

const (
	Success RegistrationResultString = "success"
	Failure RegistrationResultString = "failure"
)

type RegistrationResult struct {
	MessageType MessageType              `json:"type"` // registration
	Result      RegistrationResultString `json:"result"`
}

type UserStatus int

const (
	Waiting UserStatus = iota
	Writing
	Submitted
)

type PlayerList []Player
type Player struct {
	UserName string     `json:"user_name"`
	Status   UserStatus `json:"status"` // waiting, writing, submitted
	IsAdmin  bool       `json:"is_admin"`
}

type RoomUpdateMessage struct {
	MessageType MessageType `json:"type"` // room_update
	UserList    PlayerList  `json:"user_list"`
	RoomState   RoomState   `json:"room_state"` // lobby = 0, write_stories = 1, show_stories = 2
}

type RoundUpdateMessage struct {
	MessageType  MessageType `json:"type"` // round_update
	CurrentStage int         `json:"current_stage"`
	LastStage    int         `json:"last_stage"`
	Text         string      `json:"text"`
}

type ShowStoryMessage struct {
	MessageType MessageType `json:"type"` // show_story
	UserName    string      `json:"user_name"`
	Stories     []string    `json:"stories"`
}

type CloseRoomMessage struct {
	MessageType MessageType `json:"type"` // close_room
}

/// END Server/Client Interface

type UserStoryStage struct {
	SubmittedStories map[string]string // Maps from user name to story
	UserMapping      map[string]string // Maps from the user name that wrote the original stage to the user that is going to write the text for this stage
}

type UserStory struct {
	StoryStages        []UserStoryStage
	ParticipatingUsers map[string]bool
	LastStage          int
}

type User struct {
	Name       string
	Connection *websocket.Conn
	IsAdmin    bool
}

type RoomState int

const (
	Lobby RoomState = iota
	WriteStories
	ShowStories
)

type Room struct {
	Users     map[string]User
	RoomState RoomState
	Story     UserStory
}

type PlayerItemList []*PlayerItem
type PlayerItem struct {
	Room          string         `json:"room" dynamodbav:"room"`
	ConnectionID  string         `json:"connectionId" dynamodbav:"connectionId"`
	UserName      string         `json:"user_name" dynamodbav:"user_name"`
	Status        UserStatus     `json:"user_status" dynamodbav:"user_status"`
	IsAdmin       bool           `json:"is_admin" dynamodbav:"is_admin"`
	RoomState     RoomState      `json:"room_state" dynamodbav:"room_state"`
	LastStage     int            `json:"last_stage" dynamodbav:"last_stage"`
	Spectating    bool           `json:"spectating" dynamodbav:"spectating"`
	Contributions map[int]string `json:"contributions" dynamodbav:"contributions"`
	Participants  map[int]string `json:"participants" dynamodbav:"participants"`
	LastActivity  int64          `json:"last_activity" dynamodbav:"last_activity"`
}

func playerItemListToPlayerList(playerItems PlayerItemList) PlayerList {
	var players PlayerList
	for _, playerItem := range playerItems {
		players = append(players, Player{
			UserName: playerItem.UserName,
			Status:   playerItem.Status,
			IsAdmin:  playerItem.IsAdmin,
		})
	}
	return players
}

func marshalPlayerKey(player PlayerItem) map[string]*dynamodb.AttributeValue {
	return map[string]*dynamodb.AttributeValue{
		"room": {
			S: aws.String(player.Room),
		},
		"connectionId": {
			S: aws.String(player.ConnectionID),
		},
	}
}

func updateDynamoDBPlayerItem(db *dynamodb.DynamoDB, player PlayerItem) error {
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
			expression.IfNotExists(expression.Name(name), expression.Value(value)),
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

	_, err = db.UpdateItem(updateItemInput)
	if err != nil {
		return err
	}
	return nil
}

// Handler is the base handler that will receive all web socket request
func Handler(request events.APIGatewayWebsocketProxyRequest) (response interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("Recovered in f", r)
			response = events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}
		}
	}()

	switch request.RequestContext.RouteKey {
	case "$connect":
		err = Connect(request)
	case "$disconnect":
		err = Disconnect(request)
	default:
		err = Default(request)
	}
	if err != nil {
		log.Error(err)
		response = events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}
		return
	}

	response = events.APIGatewayProxyResponse{
		StatusCode: 200,
	}
	return
}

func getTTLTime() int64 {
	return time.Now().AddDate(0, 0, 1).Unix()
}

func postToConnection(connectionID string, sess *session.Session, data interface{}) error {
	marshalled, err := json.Marshal(data)
	if err != nil {
		return err
	}

	input := &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         marshalled,
	}

	apigateway := apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(APIGatewayEndpoint))

	_, err = apigateway.PostToConnection(input)
	if err != nil {
		return err
	}

	return nil
}

// Connect will receive the $connect request
func Connect(request events.APIGatewayWebsocketProxyRequest) error {
	log.Debug("[Connect] - Method called")
	playerItem := PlayerItem{
		Room:         DefaultRoomName,
		ConnectionID: request.RequestContext.ConnectionID,
		LastActivity: getTTLTime(),
	}
	/*attributeValues, err := dynamodbattribute.MarshalMap(playerItem)
	if err != nil {
		return err
	}

	putItemInput := &dynamodb.PutItemInput{
		TableName: aws.String(DynamoDBTable),
		Item:      attributeValues,
	}*/

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(AWSRegion),
	})
	if err != nil {
		return err
	}
	db := dynamodb.New(sess)

	/*_, err = db.PutItem(putItemInput)
		if err != nil {
			return err
	    }*/
	err = updateDynamoDBPlayerItem(db, playerItem)
	if err != nil {
		return err
	}
	log.Infof("Player with connectionId %s put into DynamoDB", request.RequestContext.ConnectionID)

	log.Debug("[Connect] - Method successfully finished")
	return nil
}

// Disconnect will receive the $disconnect requests
func Disconnect(request events.APIGatewayWebsocketProxyRequest) error {
	log.Debug("[Disconnect] - Method called")
	scanInput := &dynamodb.ScanInput{
		TableName:            aws.String(DynamoDBTable),
		ProjectionExpression: aws.String("room"),
		FilterExpression:     aws.String("connectionId = :cid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":cid": {
				S: aws.String(request.RequestContext.ConnectionID),
			},
		},
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(AWSRegion),
	})
	if err != nil {
		return err
	}
	db := dynamodb.New(sess)

	scanOutput, err := db.Scan(scanInput)
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
	log.Debugf("Player with connectionId %s is in room %s", request.RequestContext.ConnectionID, room)

	deleteItemInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(DynamoDBTable),
		Key: map[string]*dynamodb.AttributeValue{
			"room": {
				S: aws.String(room),
			},
			"connectionId": {
				S: aws.String(request.RequestContext.ConnectionID),
			},
		},
	}
	_, err = db.DeleteItem(deleteItemInput)
	if err != nil {
		return err
	}
	log.Infof("Player with connectionId %s removed from DynamoDB", request.RequestContext.ConnectionID)

	log.Debug("[Disconnect] - Method successfully finished")
	return nil
}

// Default will receive the $default request
func Default(request events.APIGatewayWebsocketProxyRequest) error {
	log.Debug("[Default] - Method called")
	var err error

	var b ClientMessage
	if err = json.NewDecoder(strings.NewReader(request.Body)).Decode(&b); err != nil {
		return err
	}
	log.Tracef("Client message received: %v", b)
	if b.Room == DefaultRoomName {
		return fmt.Errorf("Room cannot be '%s'. This is a reserved room name", DefaultRoomName)
	}

	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(DynamoDBTable),
		KeyConditionExpression: aws.String("room = :r"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":r": {
				S: aws.String(b.Room),
			},
		},
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(AWSRegion),
	})
	if err != nil {
		return err
	}
	db := dynamodb.New(sess)

	queryOutput, err := db.Query(queryInput)
	if err != nil {
		return err
	}
	log.Tracef("Query output: %v", *queryOutput)

	var players PlayerItemList
	for _, i := range queryOutput.Items {
		player := &PlayerItem{}
		if err := dynamodbattribute.UnmarshalMap(i, player); err != nil {
			log.Error(err)
			continue
		}
		players = append(players, player)
	}
	log.Debugf("Players in room %s: %v", b.Room, players)

	switch b.MessageType {
	case Registration:
		err = handleRegistration(sess, request.RequestContext.ConnectionID, b, players)
	case SubmitStory:
		//err = handleSubmitStory(b, &conn)
	case StartSession:
		//err = handleStartSession(b, &conn)
	case CloseRoom:
		//err = handleCloseRoom(b, &conn)
	case ShowStory:
		//err = handleShowStory(b, &conn)
	default:
		log.Errorf("Encountered unsupported message type %s", b.MessageType)
	}
	if err != nil {
		return err
	}

	log.Debug("[Default] - Method successfully finished")
	return nil
}

func register(db *dynamodb.DynamoDB, connectionID string, message ClientMessage, players PlayerItemList) (PlayerItemList, error) {
	player := &PlayerItem{
		Room:         message.Room,
		ConnectionID: connectionID,
		UserName:     message.UserName,
	}
	log.Debugf("Player trying to register: %v", player)

	var err error
	playerExists := false
	for _, existingPlayer := range players {
		if existingPlayer.UserName != player.UserName {
			continue
		}
		playerExists = true
		log.Debugf("Existing Player in room: %v", existingPlayer)
		oldConnectionID := existingPlayer.ConnectionID
		existingPlayer.ConnectionID = player.ConnectionID
		existingPlayer.LastActivity = getTTLTime()
		err = updateDynamoDBPlayerItem(db, *existingPlayer)
		if err != nil {
			return nil, err
		}
		deleteItemInput := &dynamodb.DeleteItemInput{
			TableName: aws.String(DynamoDBTable),
			Key: map[string]*dynamodb.AttributeValue{
				"room": {
					S: aws.String(player.Room),
				},
				"connectionId": {
					S: aws.String(oldConnectionID),
				},
			},
		}
		_, err = db.DeleteItem(deleteItemInput)
		if err != nil {
			return nil, err
		}
		log.Debugf("Old connectionId %s of player %s in room %s deleted after reconnect", oldConnectionID, player.UserName, player.Room)
	}
	if !playerExists {
		if len(players) == 0 {
			log.Infof("Player %s creates new room %s and will therefore be admin", player.UserName, player.Room)
			player.IsAdmin = true
		}
		player.LastActivity = getTTLTime()
		err = updateDynamoDBPlayerItem(db, *player)
		if err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	log.Infof("Player with connectionId %s successfully registered and assigned to room:username (%s:%s)", connectionID, player.Room, player.UserName)

	deleteItemInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(DynamoDBTable),
		Key: map[string]*dynamodb.AttributeValue{
			"room": {
				S: aws.String(DefaultRoomName),
			},
			"connectionId": {
				S: aws.String(connectionID),
			},
		},
	}
	_, err = db.DeleteItem(deleteItemInput)
	if err != nil {
		return nil, err
	}
	log.Debugf("Item with connectionId %s and %s room deleted", connectionID, DefaultRoomName)

	return players, nil
}

func handleRegistration(sess *session.Session, connectionID string, message ClientMessage, players PlayerItemList) error {
	result := RegistrationResult{
		MessageType: Registration,
		Result:      Success,
	}

	players, err := register(dynamodb.New(sess), connectionID, message, players)
	if err != nil {
		log.Error(err)
		result.Result = Failure
	}
	log.Infof("Player %s successfully registrered in room %s", message.UserName, message.Room)

	err = postToConnection(connectionID, sess, result)
	if err != nil {
		return err
	}

	if result.Result == Failure {
		log.Warnf("Not sending room update for registration of connection (%s) due to error", connectionID)
		return nil
	}

	err = sendRoomUpdate(sess, players)
	if err != nil {
		return err
	}

	return nil
}

var rooms = make(map[string]*Room)

func getTextOfStage(room *Room, ownerUserName string, stageIndex int) string {

	stage := &room.Story.StoryStages[stageIndex]
	writerName := stage.UserMapping[ownerUserName]
	return stage.SubmittedStories[writerName]
}

func handleShowStory(message *ClientMessage, connection *websocket.Conn) error {
	room, ok := rooms[message.Room]
	if !ok {
		return fmt.Errorf("Tried to show story in room that does not exist: %s", message.Room)
	}

	// Make sure only the admin closes a room
	user, ok := room.Users[message.UserName]
	if !ok {
		return fmt.Errorf("User that is not part of the room tried to show message: %s", message.UserName)
	}
	if !user.IsAdmin {
		return fmt.Errorf("User that is not an admin tried to show a message: %s", message.UserName)
	}

	var payload ShowStoryPayload
	err := json.Unmarshal([]byte(message.Payload), &payload)
	if err != nil {
		return err
	}

	if payload.Stage > room.Story.LastStage {
		payload.Stage = room.Story.LastStage
	}

	stories := make([]string, 0)
	owner := payload.UserName
	for i := 0; i < payload.Stage; i++ {
		stories = append(stories, getTextOfStage(room, owner, i))
	}

	showStoryMessage := ShowStoryMessage{
		MessageType: ShowStory,
		UserName:    payload.UserName,
		Stories:     stories,
	}

	marshalled, err := json.Marshal(showStoryMessage)
	if err != nil {
		return err
	}

	// Tell each user to show the story of that user up to that stage
	for _, user := range room.Users {
		user.Connection.WriteMessage(websocket.TextMessage, marshalled)
	}

	return nil //sendRoomUpdate(message.Room)
}

func handleCloseRoom(message *ClientMessage, connection *websocket.Conn) error {
	room, ok := rooms[message.Room]
	if !ok {
		return fmt.Errorf("Tried to close room that does not exist: %s", message.Room)
	}

	// Make sure only the admin closes a room
	user, ok := room.Users[message.UserName]
	if !ok {
		return fmt.Errorf("User that is not part of the room tried to close a room: %s", message.UserName)
	}
	if !user.IsAdmin {
		return fmt.Errorf("User that is not an admin tried to close a room: %s", message.UserName)
	}

	closeMessage := CloseRoomMessage{
		MessageType: CloseRoom,
	}

	marshalled, err := json.Marshal(closeMessage)
	if err != nil {
		return err
	}

	// Send close room message to each user
	for _, user := range room.Users {
		user.Connection.WriteMessage(websocket.TextMessage, marshalled)
	}

	// Remove room from list
	delete(rooms, message.Room)
	return nil
}

func handleStartSession(message *ClientMessage, connection *websocket.Conn) error {
	room, ok := rooms[message.Room]
	if !ok {
		return fmt.Errorf("Tried to start a session for room that does not exist: %s", message.Room)
	}

	// Make sure only the admin opens a room
	user, ok := room.Users[message.UserName]
	if !ok {
		return fmt.Errorf("User that is not part of the room tried to open a session: %s", message.UserName)
	}
	if !user.IsAdmin {
		return fmt.Errorf("User that is not an admin tried to open a session: %s", message.UserName)
	}

	room.RoomState = WriteStories
	room.Story.ParticipatingUsers = make(map[string]bool)
	for userName := range room.Users {
		room.Story.ParticipatingUsers[userName] = true
	}

	room.Story.StoryStages = make([]UserStoryStage, 0)

	userMapping := make(map[string]string)
	for userName := range room.Users {
		userMapping[userName] = userName // In the first stage we don't have a prior story --> keep 1:1 mapping
	}

	firstStage := UserStoryStage{
		SubmittedStories: make(map[string]string),
		UserMapping:      userMapping,
	}
	room.Story.StoryStages = append(room.Story.StoryStages, firstStage)

	var payload StartSessionPayload
	err := json.Unmarshal([]byte(message.Payload), &payload)
	if err != nil {
		return err
	}
	room.Story.LastStage = payload.LastStage

	// Tell each user to start with the first round
	for participant := range room.Story.ParticipatingUsers {

		text := "Start typing your story!"
		updateMessage := RoundUpdateMessage{
			MessageType:  RoundUpdate,
			CurrentStage: len(room.Story.StoryStages),
			LastStage:    room.Story.LastStage,
			Text:         text,
		}

		marshalled, err := json.Marshal(updateMessage)
		if err != nil {
			return err
		}

		user := room.Users[participant]
		user.Connection.WriteMessage(websocket.TextMessage, marshalled)
	}

	return nil //sendRoomUpdate(message.Room)
}

func handleSubmitStory(message *ClientMessage, connection *websocket.Conn) error {
	room, ok := rooms[message.Room]
	if !ok {
		return fmt.Errorf("Tried to submit a story for room that does not exist: %s", message.Room)
	}

	// Make sure that the room is in the right state
	if room.RoomState != WriteStories {
		return fmt.Errorf("Tried to submit a story for room that is currently not accepting stories: %s (Status: %v)", message.Room, room.RoomState)
	}

	// Make sure that the sender is participating in this round
	_, ok = room.Story.ParticipatingUsers[message.UserName]
	if !ok {
		return fmt.Errorf("A user that is not participating in this round tried to submit a story: %s", message.UserName)
	}

	// Accept the story
	last := &room.Story.StoryStages[len(room.Story.StoryStages)-1]
	last.SubmittedStories[message.UserName] = message.Payload

	// If this was the last story, we proceed to a new stage
	participantsCount := len(room.Story.ParticipatingUsers)
	submittedCount := len(last.SubmittedStories)

	if participantsCount == submittedCount {
		// Check if we've completed the story
		if room.Story.LastStage == len(room.Story.StoryStages) {
			// The story has been completed, show it.
			room.RoomState = ShowStories
		} else {
			// Begin the next stage
			room.Story.StoryStages = append(room.Story.StoryStages, UserStoryStage{
				SubmittedStories: make(map[string]string),
			})

			// Send the last story part to every participating user
			// At this point we know that the prior stage existed
			prior := &room.Story.StoryStages[len(room.Story.StoryStages)-2]

			participants := make([]string, len(room.Story.ParticipatingUsers))
			idx := 0
			for participant := range room.Story.ParticipatingUsers {
				participants[idx] = participant
				idx++
			}

			// Create a new permutation based on RNG
			primes := [...]int{
				2063, 2069, 2081, 2083, 2087, 2089, 2099, 2111, 2113, 2129, 2131, 2137, 2141, 2143, 2153, 2161, 2179, 2203, 2207, 2213, 2221, 2237, 2239, 2243, 2251, 2267, 2269, 2273, 2281, 2287, 2293, 2297, 2309, 2311, 2333, 2339, 2341, 2347, 2351, 2357, 2371, 2377, 2381, 2383, 2389, 2393, 2399, 2411, 2417, 2423, 2437, 2441, 2447, 2459, 2467, 2473, 2477, 2503, 2521, 2531, 2539, 2543, 2549, 2551, 2557, 2579, 2591, 2593, 2609, 2617, 2621, 2633, 2647, 2657, 2659, 2663, 2671, 2677, 2683, 2687, 2689, 2693, 2699, 2707, 2711, 2713, 2719, 2729, 2731, 2741, 2749, 2753, 2767, 2777, 2789, 2791, 2797, 2801, 2803, 2819, 2833, 2837, 2843, 2851, 2857, 2861, 2879, 2887, 2897, 2903, 2909, 2917, 2927, 2939, 2953, 2957, 2963, 2969, 2971, 2999, 3001, 3011, 3019, 3023, 3037, 3041, 3049, 3061, 3067, 3079, 3083, 3089, 3109, 3119, 3121, 3137, 3163, 3167, 3169, 3181, 3187, 3191, 3203, 3209, 3217, 3221, 3229, 3251, 3253, 3257, 3259, 3271, 3299, 3301, 3307, 3313, 3319, 3323, 3329, 3331, 3343, 3347, 3359, 3361, 3371, 3373, 3389, 3391, 3407, 3413, 3433, 3449, 3457, 3461, 3463, 3467, 3469, 3491, 3499, 3511, 3517, 3527, 3529, 3533, 3539, 3541, 3547, 3557, 3559, 3571, 3581, 3583, 3593, 3607, 3613, 3617, 3623, 3631, 3637, 3643, 3659, 3671, 3673, 3677, 3691, 3697, 3701, 3709, 3719, 3727, 3733, 3739, 3761, 3767, 3769, 3779, 3793, 3797, 3803, 3821, 3823, 3833, 3847, 3851, 3853, 3863, 3877, 3881, 3889, 3907, 3911, 3917, 3919, 3923, 3929, 3931, 3943, 3947, 3967, 3989, 4001, 4003, 4007, 4013, 4019, 4021, 4027, 4049, 4051, 4057, 4073, 4079, 4091, 4093, 4099, 4111, 4127, 4129, 4133, 4139, 4153, 4157, 4159, 4177, 4201, 4211, 4217, 4219, 4229, 4231, 4241, 4243, 4253, 4259, 4261, 4271, 4273, 4283, 4289, 4297, 4327, 4337, 4339, 4349, 4357, 4363, 4373, 4391, 4397, 4409, 4421, 4423, 4441, 4447, 4451, 4457, 4463, 4481, 4483, 4493, 4507, 4513, 4517, 4519, 4523, 4547, 4549, 4561, 4567, 4583, 4591, 4597, 4603, 4621, 4637, 4639, 4643, 4649, 4651, 4657, 4663, 4673, 4679, 4691, 4703, 4721, 4723, 4729, 4733, 4751, 4759, 4783, 4787, 4789, 4793, 4799, 4801, 4813, 4817, 4831, 4861, 4871, 4877, 4889, 4903, 4909, 4919, 4931, 4933, 4937, 4943, 4951, 4957, 4967, 4969, 4973, 4987, 4993, 4999, 5003, 5009, 5011, 5021, 5023, 5039, 5051, 5059, 5077, 5081, 5087, 5099, 5101, 5107, 5113, 5119, 5147, 5153, 5167, 5171, 5179, 5189, 5197, 5209, 5227, 5231, 5233, 5237, 5261, 5273, 5279, 5281, 5297, 5303, 5309, 5323, 5333, 5347, 5351, 5381, 5387, 5393, 5399, 5407, 5413, 5417, 5419, 5431, 5437, 5441, 5443, 5449, 5471, 5477, 5479, 5483, 5501, 5503, 5507, 5519, 5521, 5527, 5531, 5557, 5563, 5569, 5573, 5581, 5591, 5623, 5639, 5641, 5647, 5651, 5653, 5657, 5659, 5669, 5683, 5689, 5693, 5701, 5711, 5717, 5737, 5741, 5743, 5749, 5779, 5783, 5791, 5801, 5807, 5813, 5821, 5827, 5839, 5843, 5849, 5851, 5857, 5861, 5867, 5869, 5879, 5881, 5897, 5903, 5923, 5927, 5939, 5953, 5981, 5987, 6007, 6011, 6029, 6037, 6043, 6047, 6053, 6067, 6073, 6079, 6089, 6091, 6101, 6113, 6121, 6131, 6133, 6143, 6151, 6163, 6173, 6197, 6199, 6203, 6211, 6217, 6221, 6229, 6247, 6257, 6263, 6269, 6271, 6277, 6287, 6299, 6301, 6311, 6317, 6323, 6329, 6337, 6343, 6353, 6359, 6361, 6367, 6373, 6379, 6389, 6397, 6421, 6427, 6449, 6451, 6469, 6473, 6481, 6491, 6521, 6529, 6547, 6551, 6553, 6563, 6569, 6571, 6577, 6581, 6599, 6607, 6619, 6637, 6653, 6659, 6661, 6673, 6679, 6689, 6691, 6701, 6703, 6709, 6719, 6733, 6737, 6761, 6763, 6779, 6781, 6791, 6793, 6803, 6823, 6827, 6829, 6833, 6841, 6857, 6863, 6869, 6871, 6883, 6899, 6907, 6911, 6917, 6947, 6949, 6959, 6961, 6967, 6971, 6977, 6983, 6991, 6997, 7001, 7013, 7019, 7027, 7039, 7043, 7057, 7069, 7079, 7103, 7109, 7121, 7127, 7129, 7151, 7159, 7177, 7187, 7193, 7207, 7211, 7213, 7219, 7229, 7237, 7243, 7247, 7253, 7283, 7297, 7307, 7309, 7321, 7331, 7333, 7349, 7351, 7369, 7393, 7411, 7417, 7433, 7451, 7457, 7459, 7477, 7481, 7487, 7489, 7499, 7507, 7517, 7523, 7529, 7537, 7541, 7547, 7549, 7559, 7561, 7573, 7577, 7583, 7589, 7591, 7603, 7607, 7621, 7639, 7643, 7649, 7669, 7673, 7681, 7687, 7691, 7699, 7703, 7717, 7723, 7727, 7741, 7753, 7757, 7759, 7789, 7793, 7817, 7823, 7829, 7841, 7853, 7867, 7873, 7877, 7879, 7883, 7901, 7907, 7919,
			}

			// Choose a random prime number
			primeIndex := rand.Intn(len(primes))

			prime := primes[primeIndex]
			// If we would map to the same index, we have to choose a different prime number
			if (prime % participantsCount) == 0 {
				primeIndex = (primeIndex + 1) % len(primes)
				prime = primes[primeIndex]
			}

			oldSubmitterToNewSubmitterMapping := make(map[string]string)
			for i, oldSubmitter := range participants {
				newSubmitterIndex := (i + prime) % participantsCount
				newSubmitter := participants[newSubmitterIndex]
				oldSubmitterToNewSubmitterMapping[oldSubmitter] = newSubmitter
			}

			last = &room.Story.StoryStages[len(room.Story.StoryStages)-1]
			last.UserMapping = make(map[string]string)
			for _, owner := range participants {
				oldSubmitter := prior.UserMapping[owner]
				newSubmitter := oldSubmitterToNewSubmitterMapping[oldSubmitter]
				last.UserMapping[owner] = newSubmitter
			}

			// Send the old stories to the participating users
			for _, owner := range participants {
				text := getTextOfStage(room, owner, len(room.Story.StoryStages)-2)

				// Find out who is going to write the next stage of the owners story
				receiverUserName := last.UserMapping[owner]
				receiver := room.Users[receiverUserName]

				updateMessage := RoundUpdateMessage{
					MessageType:  RoundUpdate,
					CurrentStage: len(room.Story.StoryStages),
					LastStage:    room.Story.LastStage,
					Text:         text,
				}

				marshalled, err := json.Marshal(updateMessage)
				if err != nil {
					return err
				}

				receiver.Connection.WriteMessage(websocket.TextMessage, marshalled)
			}
		}
	}
	return nil //sendRoomUpdate(message.Room) // Something changed in this room so we immediately send an update
}

func sendRoomUpdate(sess *session.Session, playerItems PlayerItemList) error {
	if len(playerItems) == 0 {
		return fmt.Errorf("Tried to send update for room that does not exist")
	}

	message := RoomUpdateMessage{
		MessageType: RoomUmdate,
		RoomState:   playerItems[0].RoomState,
		UserList:    playerItemListToPlayerList(playerItems),
	}

	log.Debugf("Sending room update to room %s: %v", playerItems[0].Room, message)

	var errors []error
	// Send the room update to every user in the room
	for _, player := range playerItems {
		err := postToConnection(player.ConnectionID, sess, message)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("%v", errors)
	}

	return nil
}
