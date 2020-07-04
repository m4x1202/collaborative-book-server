package main

import (
	"encoding/json"
	"fmt"
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

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
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
	ShowLobby   bool     `json:"show_lobby"`
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
	Stage            int
}

type UserStory struct {
	StoryStages        []UserStoryStage
	ParticipatingUsers map[string]bool
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

var rooms = make(map[string]Room)

func handleShowStory(message *ClientMessage, connection *websocket.Conn) error {
	return fmt.Errorf("TODO: Write handleShowStory")
}

func handleCloseRoom(message *ClientMessage, connection *websocket.Conn) error {
	return fmt.Errorf("TODO: Write handleCloseRoom")
}

func handleStartSession(message *ClientMessage, connection *websocket.Conn) error {
	return fmt.Errorf("TODO: Write handleStartSession")
}

func handleSubmitStory(message *ClientMessage, connection *websocket.Conn) error {
	return fmt.Errorf("TODO: Write handleSubmitStory")
}

func sendRoomUpdate(roomName string) error {

	room, ok := rooms[roomName]
	if !ok {
		return fmt.Errorf("Tried to send update for room that does not exist: %s", roomName)
	}

	message := RoomUpdateMessage{
		MessageType: "room_update",
		ShowLobby:   room.RoomState == RoomStateLobby,
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
		rooms[message.Room] = Room{
			Users:     make(map[string]User),
			Story:     story,
			RoomState: RoomStateLobby,
		}
		room = rooms[message.Room]
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
			return
		}
	}
}
