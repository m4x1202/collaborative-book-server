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
	UserName string
	Story    string
}

type Room struct {
	StoryStages []UserStoryStage
	UserMap     map[string]*websocket.Conn
}

var openConnectionMutex sync.Mutex
var openConnections = make(map[string]map[string]*websocket.Conn)

func debugPrintOpenConnections() {
	for room, userMap := range openConnections {
		for name, connection := range userMap {
			log.Tracef("Room: %s, Name: %s, Connection %v", room, name, connection)
		}
	}
}

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

func sendConnectedUsersUpdate(room string) error {
	message := RoomUpdateMessage{
		MessageType: "user_update",
	}

	// TODO set is_admin true for first user
	for userName := range openConnections[room] {
		message.UserList = append(message.UserList, Player{UserName: userName})
	}

	marshalled, err := json.Marshal(message)
	if err != nil {
		return err
	}

	for _, connection := range openConnections[room] {
		connection.WriteMessage(websocket.TextMessage, marshalled)
	}

	return nil
}

func register(message *ClientMessage, connection *websocket.Conn) RegistrationResult {

	openConnectionMutex.Lock()
	defer openConnectionMutex.Unlock()

	if openConnections[message.Room] == nil {
		openConnections[message.Room] = make(map[string]*websocket.Conn)
	}
	openConnections[message.Room][message.UserName] = connection

	log.Printf("Registered user %s in room %s", message.UserName, message.Room)

	debugPrintOpenConnections()

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

	err = sendConnectedUsersUpdate(message.Room)
	if err != nil {
		return err
	}

	return nil
}

func handleConnection(conn *websocket.Conn) error {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}

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
