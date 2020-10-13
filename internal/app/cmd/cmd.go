package cmd

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	cb "github.com/m4x1202/collaborative-book"
	"github.com/m4x1202/collaborative-book/internal/app/apigateway"
	"github.com/m4x1202/collaborative-book/internal/app/dynamodb"
	log "github.com/sirupsen/logrus"
)

func Run() int {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetReportCaller(true)
	log.SetLevel(log.InfoLevel)

	lambda.Start(Handler)
	return 0
}

// Handler is the base handler that will receive all web socket request
func Handler(request events.APIGatewayWebsocketProxyRequest) (response interface{}, err error) {
	log.Trace(request)

	defer func() {
		if r := recover(); r != nil {
			log.Error("Recovered in f", r)
			response = events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}
		}
	}()

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-central-1"),
	})
	if err != nil {
		log.Error(err)
		response = events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}
		return
	}
	dbService := dynamodb.NewDBService(sess)
	wsService := apigateway.NewWSService(sess)

	switch request.RequestContext.RouteKey {
	case "$disconnect":
		err = Disconnect(dbService, request)
	default:
		log.Debugf("Route Key: %s", request.RequestContext.RouteKey)
		err = Default(dbService, wsService, request)
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

// Disconnect will receive the $disconnect requests
func Disconnect(dbs cb.DBService, request events.APIGatewayWebsocketProxyRequest) error {
	log.Debug("[Disconnect] - Method called")
	err := dbs.RemoveConnection(request.RequestContext.ConnectionID)
	if err != nil {
		return err
	}
	log.Debug("[Disconnect] - Method successfully finished")
	return nil
}

// Default will receive the $default request
func Default(dbs cb.DBService, wss cb.WSService, request events.APIGatewayWebsocketProxyRequest) error {
	log.Debug("[Default] - Method called")
	var err error

	var b cb.ClientMessage
	if err = json.NewDecoder(strings.NewReader(request.Body)).Decode(&b); err != nil {
		return err
	}
	log.Tracef("Client message received: %v", b)
	err = b.Sanitize()
	if err != nil {
		return err
	}

	players, err := dbs.GetPlayerItems(b.Room)
	if err != nil {
		return err
	}
	log.Debugf("Players in room %s: %v", b.Room, players)

	if b.MessageType == cb.Registration {
		err = handleRegistration(dbs, wss, request.RequestContext.ConnectionID, b, players)
	} else {
		if len(players) == 0 {
			return fmt.Errorf("Tried to apply an action on room that does not exist: %s", b.Room)
		}
		switch b.MessageType {
		case cb.SubmitStory:
			err = handleSubmitStory(dbs, wss, b, players)
		case cb.StartSession:
			err = handleStartSession(dbs, wss, b, players)
		case cb.CloseRoom:
			err = handleCloseRoom(dbs, wss, b, players)
		case cb.ShowStory:
			err = handleShowStory(wss, b, players)
		default:
			err = fmt.Errorf("Encountered unsupported message type %s", b.MessageType)
		}
	}
	if err != nil {
		return err
	}

	log.Debug("[Default] - Method successfully finished")
	return nil
}

func register(dbs cb.DBService, connectionID string, message cb.ClientMessage, players cb.PlayerItemList) (cb.PlayerItemList, error) {
	playerInfo := cb.PlayerInfo{
		UserName:   message.UserName,
		Spectating: true,
		Status:     cb.Waiting,
	}
	player := &cb.PlayerItem{
		Room:         message.Room,
		ConnectionID: connectionID,
		PlayerInfo:   &playerInfo,
	}
	log.Debugf("Player trying to register: %v", player)

	var err error
	playerExists := false
	for _, existingPlayer := range players {
		if existingPlayer.PlayerInfo.UserName != player.PlayerInfo.UserName {
			continue
		}
		playerExists = true
		log.Debugf("Existing Player in room: %v", existingPlayer)
		if existingPlayer.ConnectionID != player.ConnectionID {
			err = dbs.RemovePlayerItem(*existingPlayer)
			if err != nil {
				return nil, err
			}
			log.Debugf("Old connection_id %s of player %s in room %s deleted after reconnect", existingPlayer.ConnectionID, existingPlayer.PlayerInfo.UserName, existingPlayer.Room)
			existingPlayer.ConnectionID = player.ConnectionID
		}
		player = existingPlayer
	}
	if !playerExists {
		players = append(players, player)
	}
	if players.GetAdmin() == nil {
		log.Infof("Room %s does not yet have an admin. New admin is user %s", player.Room, player.PlayerInfo.UserName)
		player.PlayerInfo.IsAdmin = true
	}
	err = dbs.UpdatePlayerItem(player)
	if err != nil {
		return nil, err
	}
	log.Infof("Player with connection_id %s successfully registered and assigned to room:username (%s:%s)", connectionID, player.Room, player.PlayerInfo.UserName)

	err = dbs.RemovePlayerItem(cb.PlayerItem{
		Room:         cb.DefaultRoomName,
		ConnectionID: connectionID,
	})
	if err != nil {
		return nil, err
	}
	log.Debugf("Item with connection_id %s and %s room deleted", connectionID, cb.DefaultRoomName)

	return players, nil
}

func handleRegistration(dbs cb.DBService, wss cb.WSService, connectionID string, message cb.ClientMessage, players cb.PlayerItemList) error {
	result := cb.RegistrationResult{
		MessageType: cb.Registration,
		Result:      cb.Success,
	}

	players, err := register(dbs, connectionID, message, players)
	if err != nil {
		log.Error(err)
		result.Result = cb.Failure
	}
	log.Infof("Player %s successfully registrered in room %s", message.UserName, message.Room)

	err = wss.PostToConnection(connectionID, result)
	if err != nil {
		return err
	}

	if result.Result == cb.Failure {
		log.Warnf("Not sending room update for registration of connection (%s) due to error", connectionID)
		return nil
	}

	return sendRoomUpdate(wss, players)
}

func sendRoomUpdate(wss cb.WSService, playerItems cb.PlayerItemList) error {
	if len(playerItems) == 0 {
		return fmt.Errorf("Tried to send update for room that does not exist")
	}

	message := cb.RoomUpdateMessage{
		MessageType: cb.RoomUmdate,
		RoomState:   playerItems.GetAdmin().PlayerInfo.RoomState,
		UserList:    playerItems.PlayerItemListToPlayerList(),
	}

	log.Debugf("Sending room update to room %s: %v", playerItems[0].Room, message)

	return wss.PostToConnections(playerItems.GetConnectionIDsOfPlayerItems(), message)
}

func handleStartSession(dbs cb.DBService, wss cb.WSService, message cb.ClientMessage, players cb.PlayerItemList) error {
	messagingPlayer := players.GetPlayerItemFromUserName(message.UserName)
	if messagingPlayer == nil {
		return fmt.Errorf("User %s that is not part of room %s tried to open a session", message.UserName, message.Room)
	}
	// Make sure only the admin opens a room
	if !messagingPlayer.PlayerInfo.IsAdmin {
		return fmt.Errorf("User %s that is not an admin in room %s tried to open a session", message.UserName, message.Room)
	}

	var err error
	var payload cb.StartSessionPayload
	err = json.Unmarshal([]byte(message.Payload), &payload)
	if err != nil {
		return err
	}
	// Make sure there is at least 1 stage to play
	if payload.LastStage < 1 {
		log.Warn("Stages to play smaller than 1! This shouldn't happen. Setting to fixed minimum value 1")
		payload.LastStage = 1
	}

	participants := GenerateParticipants(players, payload.LastStage)

	for _, player := range players {
		playerInfo := player.PlayerInfo
		if playerInfo.Participants == nil {
			playerInfo.Participants = participants[playerInfo.UserName].ToStringMap()
		}
		playerInfo.LastStage = payload.LastStage
		playerInfo.RoomState = cb.WriteStories
		playerInfo.Status = cb.Writing
		playerInfo.Spectating = false

		err = dbs.UpdatePlayerItem(player)
		if err != nil {
			return err
		}
	}

	updateMessage := cb.RoundUpdateMessage{
		MessageType:  cb.RoundUpdate,
		CurrentStage: 1,
		LastStage:    payload.LastStage,
		Text:         "Start typing your story!",
	}
	err = wss.PostToConnections(players.GetConnectionIDsOfPlayerItems(), updateMessage)
	if err != nil {
		return err
	}

	return sendRoomUpdate(wss, players)
}

func handleCloseRoom(dbs cb.DBService, wss cb.WSService, message cb.ClientMessage, players cb.PlayerItemList) error {
	messagingPlayer := players.GetPlayerItemFromUserName(message.UserName)
	if messagingPlayer == nil {
		return fmt.Errorf("User %s that is not part of room %s tried to close the room", message.UserName, message.Room)
	}
	// Make sure only the admin closes a room
	if !messagingPlayer.PlayerInfo.IsAdmin {
		return fmt.Errorf("User %s that is not an admin in room %s tried to close the room", message.UserName, message.Room)
	}

	var err error
	for _, player := range players {
		err = dbs.ResetPlayerItem(player)
		if err != nil {
			return err
		}
	}

	closeMessage := cb.CloseRoomMessage{
		MessageType: cb.CloseRoom,
	}

	err = wss.PostToConnections(players.GetConnectionIDsOfPlayerItems(), closeMessage)
	if err != nil {
		return err
	}
	return nil
}

func handleShowStory(wss cb.WSService, message cb.ClientMessage, players cb.PlayerItemList) error {
	messagingPlayer := players.GetPlayerItemFromUserName(message.UserName)
	if messagingPlayer == nil {
		return fmt.Errorf("User %s that is not part of room %s tried to show a message", message.UserName, message.Room)
	}
	// Make sure only the admin closes a room
	if !messagingPlayer.PlayerInfo.IsAdmin {
		return fmt.Errorf("User %s that is not an admin in room %s tried to show a message", message.UserName, message.Room)
	}

	var err error
	var payload cb.ShowStoryPayload
	err = json.Unmarshal([]byte(message.Payload), &payload)
	if err != nil {
		return err
	}

	playerOfStory := players.GetPlayerItemFromUserName(payload.UserName)
	if payload.Stage > playerOfStory.PlayerInfo.LastStage {
		payload.Stage = playerOfStory.PlayerInfo.LastStage
	}

	stories := make([]string, 0, payload.Stage)
	for stage := 1; stage <= payload.Stage; stage++ {
		stageStr := strconv.Itoa(stage)
		stories = append(stories, players.GetPlayerItemFromUserName(playerOfStory.PlayerInfo.Participants[stageStr]).PlayerInfo.Contributions[stageStr])
	}

	showStoryMessage := cb.ShowStoryMessage{
		MessageType: cb.ShowStory,
		UserName:    payload.UserName,
		Stories:     stories,
	}

	return wss.PostToConnections(players.GetConnectionIDsOfPlayerItems(), showStoryMessage)
}

func handleSubmitStory(dbs cb.DBService, wss cb.WSService, message cb.ClientMessage, players cb.PlayerItemList) error {
	messagingPlayer := players.GetPlayerItemFromUserName(message.UserName)
	if messagingPlayer == nil {
		return fmt.Errorf("User %s that is not part of room %s tried to submit a story", message.UserName, message.Room)
	}
	// Make sure that the sender is participating in this session
	if messagingPlayer.PlayerInfo.Spectating {
		return fmt.Errorf("User %s is only spectating room %s but tried to submit a story", message.UserName, message.Room)
	}
	// Make sure that the room is in the right state
	if messagingPlayer.PlayerInfo.RoomState != cb.WriteStories {
		return fmt.Errorf("User %s tried to submit a story for room %s that is currently not accepting stories (Status: %v)", message.UserName, message.Room, messagingPlayer.PlayerInfo.RoomState)
	}

	var err error

	currentStage := 1 + len(messagingPlayer.PlayerInfo.Contributions)
	if messagingPlayer.PlayerInfo.Contributions == nil {
		messagingPlayer.PlayerInfo.Contributions = make(map[string]string, messagingPlayer.PlayerInfo.LastStage)
	}
	messagingPlayer.PlayerInfo.Contributions[strconv.Itoa(currentStage)] = message.Payload
	messagingPlayer.PlayerInfo.Status = cb.Submitted
	err = dbs.UpdatePlayerItem(messagingPlayer)
	if err != nil {
		return err
	}

	for _, player := range players {
		if player.PlayerInfo.Spectating {
			continue
		}
		// If this was not the last story, send roomUpdate and return
		if player.PlayerInfo.Status != cb.Submitted {
			return sendRoomUpdate(wss, players)
		}
	}
	// If it was, we proceed to a new stage

	// Check if we've completed the story
	if messagingPlayer.PlayerInfo.LastStage == currentStage {
		// The story has been completed, show it.
		for _, player := range players {
			player.PlayerInfo.RoomState = cb.ShowStories
			err = dbs.UpdatePlayerItem(player)
			if err != nil {
				return err
			}
		}
		return sendRoomUpdate(wss, players)
	}

	// Begin the next stage
	for _, player := range players {
		player.PlayerInfo.Status = cb.Writing
		err = dbs.UpdatePlayerItem(player)
		if err != nil {
			return err
		}

		lastStory := players.GetLastStory(player.PlayerInfo.UserName, strconv.Itoa(currentStage))
		if lastStory == "" {
			log.Errorf("Could not find last story for user %s in current stage %d in room %s", player.PlayerInfo.UserName, currentStage+1, player.Room)
		}
		updateMessage := cb.RoundUpdateMessage{
			MessageType:  cb.RoundUpdate,
			CurrentStage: currentStage,
			LastStage:    player.PlayerInfo.LastStage,
			Text:         lastStory,
		}
		err = wss.PostToConnection(player.ConnectionID, updateMessage)
		if err != nil {
			return err
		}
	}
	return sendRoomUpdate(wss, players) // Something changed in this room so we immediately send an update
}

func GenerateParticipants(players cb.PlayerItemList, numStages int) map[string]Participants {
	numPlayers := len(players)
	result := make(map[string]Participants, numPlayers)
	availableUsers := make([]string, 0, numPlayers)

	for _, player := range players {
		availableUsers = append(availableUsers, player.PlayerInfo.UserName)
		result[player.PlayerInfo.UserName] = make(Participants, numStages)
		result[player.PlayerInfo.UserName][1] = player.PlayerInfo.UserName
	}
	if numStages == 1 {
		return result
	}

	rand.Seed(time.Now().UnixNano())

	for stage := 2; stage <= numStages; stage++ {
		if numPlayers == 1 {
			result[players[0].PlayerInfo.UserName][stage] = players[0].PlayerInfo.UserName
			continue
		}

		remainingPlayers := make([]string, numPlayers, numPlayers)
		copy(remainingPlayers, availableUsers)
		rand.Shuffle(len(remainingPlayers), func(i, j int) { remainingPlayers[i], remainingPlayers[j] = remainingPlayers[j], remainingPlayers[i] })
		for _, player := range players {
			success := false
			for !success {
				// Ensure we do not assign a player twice to the same story
				if result[player.PlayerInfo.UserName][stage-1] == remainingPlayers[0] {
					rand.Shuffle(len(remainingPlayers), func(i, j int) { remainingPlayers[i], remainingPlayers[j] = remainingPlayers[j], remainingPlayers[i] })
					break
				}
				// Ensure there is always more 1 stage distance between assignments
				if numPlayers > 2 && stage >= 3 && result[player.PlayerInfo.UserName][stage-2] == remainingPlayers[0] {
					rand.Shuffle(len(remainingPlayers), func(i, j int) { remainingPlayers[i], remainingPlayers[j] = remainingPlayers[j], remainingPlayers[i] })
					break
				}
				result[player.PlayerInfo.UserName][stage] = remainingPlayers[0]
				remainingPlayers = remainingPlayers[1:]
				success = true
			}
		}
	}

	return result
}

type Participants map[int]string

func (in Participants) ToStringMap() map[string]string {
	out := make(map[string]string, len(in))
	for key, val := range in {
		out[strconv.Itoa(key)] = val
	}
	return out
}
