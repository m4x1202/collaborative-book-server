package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func main() {
	// serve static files
	client := gin.Default()
	client.Static("/", "./client")
	go client.Run(":8080")

	// websocket
	server := gin.Default()
	server.GET("/", func(c *gin.Context) {
		go handleWebsocket(c.Writer, c.Request)
	})
	server.Run(":8081")
}

/// BEGIN Server/Client Interface

/**
 * Possible MessageTypes with according payload:
 * 	registration()
 * 	start_session(number of stages on start)
 * 	close_room()
 * 	submit_story(story text)
 * 	show_story(user name and stage to show the story from)
 */

type ShowStoryPayload struct {
	UserName string `json:"user_name"`
	Stage    int    `json:"stage"`
}

type StartSessionPayload struct {
	LastStage int `json:"last_stage"`
}

type ClientMessage struct {
	MessageType string `json:"type"` // registration, start_session, close_room, submit_story, show_story
	Room        string `json:"room"`
	UserName    string `json:"name"`
	Payload     string `json:"payload"`
}

type RegistrationResult struct {
	MessageType string `json:"type"` // registration
	Result      string `json:"result"`
}

type Player struct {
	UserName string `json:"user_name"`
	Status   string `json:"status"`
	IsAdmin  bool   `json:"is_admin"`
}

type RoomUpdateMessage struct {
	MessageType string   `json:"type"` // room_update
	UserList    []Player `json:"user_list"`
	RoomState   int      `json:"room_state"` // lobby = 0, write_stories = 1, show_stories = 2
}

type RoundUpdateMessage struct {
	MessageType  string `json:"type"` // round_update
	CurrentStage int    `json:"current_stage"`
	LastStage    int    `json:"last_stage"`
	Text         string `json:"text"`
}

type ShowStoryMessage struct {
	MessageType string   `json:"type"` // show_story
	UserName    string   `json:"user_name"`
	Stories     []string `json:"stories"`
}

type CloseRoomMessage struct {
	MessageType string `json:"type"` // close_room
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

const (
	RoomStateLobby        = 0
	RoomStateWriteStories = 1
	RoomStateShowStories  = 2
)

type Room struct {
	Users     map[string]User
	RoomState int
	Story     UserStory
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
		MessageType: "show_story",
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

	return sendRoomUpdate(message.Room)
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
		MessageType: "close_room",
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

	room.RoomState = RoomStateWriteStories
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
			MessageType:  "round_update",
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

	return sendRoomUpdate(message.Room)
}

func handleSubmitStory(message *ClientMessage, connection *websocket.Conn) error {
	room, ok := rooms[message.Room]
	if !ok {
		return fmt.Errorf("Tried to submit a story for room that does not exist: %s", message.Room)
	}

	// Make sure that the room is in the right state
	if room.RoomState != RoomStateWriteStories {
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
			room.RoomState = RoomStateShowStories
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
			prime := 4447
			// If we would map to the same index, we have to choose a different prime number
			if (prime % participantsCount) == 0 {
				prime = 7823
			}
			offset := rand.Intn(participantsCount)

			oldSubmitterToNewSubmitterMapping := make(map[string]string)
			for i, oldSubmitter := range participants {
				newSubmitterIndex := (i + offset + prime) % participantsCount
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
					MessageType:  "round_update",
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
	return sendRoomUpdate(message.Room) // Something changed in this room so we immediately send an update
}

func sendRoomUpdate(roomName string) error {

	room, ok := rooms[roomName]
	if !ok {
		return fmt.Errorf("Tried to send update for room that does not exist: %s", roomName)
	}

	message := RoomUpdateMessage{
		MessageType: "room_update",
		RoomState:   room.RoomState,
		UserList:    make([]Player, 0),
	}

	// Create the player for each connected user
	for _, user := range room.Users {

		status := "waiting" // default state in lobby or during show stories

		// if round is going on and the user is in the participating users
		if room.RoomState == RoomStateWriteStories {

			// check if the user takes part in this round
			_, ok := room.Story.ParticipatingUsers[user.Name]
			if ok {
				// find last entry into the story stages
				last := room.Story.StoryStages[len(room.Story.StoryStages)-1]

				_, ok := last.SubmittedStories[user.Name]
				if ok {
					// if the user has something submitted already
					status = "submitted"
				} else {
					// if the user has not submitted a story yet
					status = "writing"
				}
			}
		}

		player := Player{
			UserName: user.Name,
			IsAdmin:  user.IsAdmin,
			Status:   status,
		}

		message.UserList = append(message.UserList, player)
	}

	marshalled, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Send the room update to every user in the room
	for _, user := range room.Users {
		user.Connection.WriteMessage(websocket.TextMessage, marshalled)
	}

	return nil
}

func register(message *ClientMessage, connection *websocket.Conn) RegistrationResult {
	room, ok := rooms[message.Room]
	if !ok {
		story := UserStory{
			StoryStages:        make([]UserStoryStage, 0),
			ParticipatingUsers: make(map[string]bool),
		}

		room = new(Room)
		room.Users = make(map[string]User)
		room.Story = story
		room.RoomState = RoomStateLobby
		rooms[message.Room] = room
	}

	// Make the new user the admin if he's the first to enter the room.
	isAdmin := len(room.Users) == 0

	// Add a user to the room. Overwrite with new connection if the user already existed.
	room.Users[message.UserName] = User{
		Name:       message.UserName,
		Connection: connection,
		IsAdmin:    isAdmin,
	}

	log.Printf("Registered user %s in room %s", message.UserName, message.Room)

	result := RegistrationResult{
		MessageType: "registration",
		Result:      "success",
	}
	return result
}

func handleRegistration(message *ClientMessage, connection *websocket.Conn) error {
	result := register(message, connection)
	marshalled, err := json.Marshal(result)
	if err != nil {
		return err
	}

	connection.WriteMessage(websocket.TextMessage, marshalled)

	err = sendRoomUpdate(message.Room)
	if err != nil {
		return err
	}

	return nil
}

var mutex sync.Mutex

func handleConnection(conn *websocket.Conn) error {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	// TODO: Do finer grained locking later. For now serialize message handling.
	mutex.Lock()
	defer mutex.Unlock()

	var message ClientMessage
	err = json.Unmarshal(msg, &message)
	if err != nil {
		return err
	}

	switch message.MessageType {
	case "registration":
		err = handleRegistration(&message, conn)
		if err != nil {
			return err
		}
	case "submit_story":
		err = handleSubmitStory(&message, conn)
		if err != nil {
			return err
		}
	case "start_session":
		err = handleStartSession(&message, conn)
		if err != nil {
			return err
		}
	case "close_room":
		err = handleCloseRoom(&message, conn)
		if err != nil {
			return err
		}
	case "show_story":
		err = handleShowStory(&message, conn)
		if err != nil {
			return err
		}
	default:
		log.Error("Encountered unsupported message type %s", message.MessageType)
	}
	return nil
}

var wsupgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err)
		return
	}

	for {
		err = handleConnection(conn)
		if err != nil {
			log.Error(err)
			break
		}
	}

	conn.Close()
}
