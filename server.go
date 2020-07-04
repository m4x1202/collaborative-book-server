package main

import (
	"encoding/json"
	"net/http"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func main() {
	client := gin.Default()

	// serve static files under localhost:8080/assets - this is for css and js
	client.Static("/", "./client")

	go client.Run(":8080") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")

	server := gin.Default()

	// serve the client websocket
	server.GET("/", func(c *gin.Context) {
		go wshandler(c.Writer, c.Request)
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

func register(message *ClientMessage, connection *websocket.Conn) (RegistrationResult, error) {

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
	return result, nil
}

func submitStory(message *ClientMessage) {

}

func sendConnectedUsersUpdate(messageType int, room string) error {
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
		connection.WriteMessage(messageType, marshalled)
	}

	return nil
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err)
		return
	}

	for {
		t, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		log.Print("In Loop")

		var message ClientMessage
		err = json.Unmarshal(msg, &message)
		if err != nil {
			log.Error(err)
			return
		}

		switch message.MessageType {
		case "registration":
			result, err := register(&message, conn)
			if err != nil {
				log.Error(err)
				return
			}

			marshalled, err := json.Marshal(result)
			if err != nil {
				log.Error(err)
				return
			}

			conn.WriteMessage(t, marshalled)

			err = sendConnectedUsersUpdate(t, message.Room)
			if err != nil {
				log.Error(err)
				return
			}

		case "submit_story":
			submitStory(&message)

			// Use conn to send and receive messages.
			conn.WriteMessage(t, msg)
		}
	}

	log.Print("End")

}
