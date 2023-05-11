package cb

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultRoomName = "unknown"
)

type RoomState int

const (
	Lobby RoomState = iota
	WriteStories
	ShowStories
)

/// BEGIN Server/Client Interface

type MessageType string

func (mt MessageType) In(mtl []MessageType) bool {
	for _, t := range mtl {
		if t == mt {
			return true
		}
	}
	return false
}

var (
	ClientMessageTypes = [...]MessageType{
		Registration,
		StartSession,
		CloseRoom,
		SubmitStory,
		ShowStory,
	}
	ServerMessageTypes = [...]MessageType{
		Registration,
		RoomUmdate,
		RoundUpdate,
		ShowStory,
		CloseRoom,
	}
)

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
	Payload     string      `json:"payload,omitempty"`
}

func (cm *ClientMessage) Sanitize() error {
	// Ensure room is case-insensitive
	cm.Room = strings.ToLower(cm.Room)
	if cm.Room == DefaultRoomName {
		return fmt.Errorf("Room cannot be '%s'. This is a reserved room name", DefaultRoomName)
	}

	if !cm.MessageType.In(ClientMessageTypes[:]) {
		return fmt.Errorf("ClientMessage message type unknown")
	}
	return nil
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

type UserStatus string

const (
	Waiting    UserStatus = "waiting"
	Writing    UserStatus = "writing"
	Submitted  UserStatus = "submitted"
	Spectating UserStatus = "spectating"
)

type PlayerList []Player
type Player struct {
	UserName string     `json:"user_name"`
	Status   UserStatus `json:"status"` // waiting, writing, submitted
	Type     PlayerType `json:"type"`
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

/// DynamoDB item

type PlayerType int

const (
	TPlayer PlayerType = iota
	TSpectator
	TAdmin
)

type PlayerInfo struct {
	UserName      string            `json:"user_name"`
	Status        UserStatus        `json:"user_status"`
	Type          PlayerType        `json:"player_type"`
	RoomState     RoomState         `json:"room_state"`
	LastStage     int               `json:"last_stage"`
	Contributions map[string]string `json:"contributions"`
	Participants  map[string]string `json:"participants"`
}

func (m *PlayerInfo) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	j, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	log.Debug(string(j))
	return attributevalue.Marshal(string(j))
}

func (u *PlayerInfo) UnmarshalDynamoDBAttributeValue(av types.AttributeValue) error {
	var unmarshalled string
	if err := attributevalue.Unmarshal(av, &unmarshalled); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(unmarshalled), u); err != nil {
		return err
	}
	return nil
}

type PlayerItem struct {
	Room           string      `dynamodbav:"room"`
	ConnectionID   string      `dynamodbav:"connection_id"`
	PlayerInfo     *PlayerInfo `dynamodbav:"player_info"`
	ExpirationTime int64       `dynamodbav:"expiration_time"`
}

type PlayerItemList []*PlayerItem

func (pil PlayerItemList) GetAdmin() *PlayerItem {
	for _, player := range pil {
		if player.PlayerInfo.Type == TAdmin {
			return player
		}
	}
	return nil
}

func (pil PlayerItemList) GetPlayerNames() []string {
	playerNames := make([]string, len(pil))
	for i, playerItem := range pil {
		playerNames[i] = playerItem.PlayerInfo.UserName
	}
	return playerNames
}

func (pil PlayerItemList) GetConnectionIDsOfPlayerItems() []string {
	connectionIDs := make([]string, 0, len(pil))
	for _, player := range pil {
		connectionIDs = append(connectionIDs, player.ConnectionID)
	}
	return connectionIDs
}
func (pil PlayerItemList) GetPlayerItemFromUserName(userName string) *PlayerItem {
	for _, playerItem := range pil {
		if playerItem.PlayerInfo.UserName == userName {
			return playerItem
		}
	}
	return nil
}
func (pil PlayerItemList) PlayerItemListToPlayerList() PlayerList {
	players := make([]Player, 0, len(pil))
	for _, playerItem := range pil {
		playerInfo := playerItem.PlayerInfo
		players = append(players, Player{
			UserName: playerInfo.UserName,
			Status:   playerInfo.Status,
			Type:     playerInfo.Type,
		})
	}
	return players
}

func (pil PlayerItemList) GetLastStory(userName string, currentStage string) string {
	currentStageInt, _ := strconv.Atoi(currentStage)
	nextStage := strconv.Itoa(currentStageInt + 1)
	for _, player := range pil {
		if player.PlayerInfo.Participants[nextStage] == userName {
			return pil.GetPlayerItemFromUserName(player.PlayerInfo.Participants[currentStage]).PlayerInfo.Contributions[currentStage]
		}
	}
	return ""
}

type DBService interface {
	UpdatePlayerItem(player *PlayerItem) error
	ResetPlayerItem(player *PlayerItem) error
	RemovePlayerItem(player PlayerItem) error
	RemoveConnection(connectionID string) error
	GetPlayerItems(room string) (PlayerItemList, error)
}

type WSService interface {
	PostToConnection(connectionID string, data interface{}) error
	PostToConnections(connectionIDs []string, data interface{}) error
}

type ParticipantsFactory interface {
	Generate(numStages int) (map[string]Participants, error)
}

type Participants map[int]string

func (in Participants) ToStringMap() map[string]string {
	out := make(map[string]string, len(in))
	for key, val := range in {
		out[strconv.Itoa(key)] = val
	}
	return out
}

func (in Participants) ParticipantWouldMeetConditions(player string, numPlayers int) bool {
	stage := len(in) + 1
	newParticipants := make(Participants, stage)
	for prevStage, participant := range in {
		newParticipants[prevStage] = participant
	}
	newParticipants[stage] = player
	return newParticipants.ConditionsMet(numPlayers)
}

func (in Participants) ConditionsMet(numPlayers int) bool {
	if len(in) <= 1 {
		return true
	}
	for stage := 2; stage <= len(in); stage++ {
		if numPlayers <= 1 {
			// This check shouldn't be neccessary, but to ensure everything is correct we still do it
			// Also to verify the results of the benchmark
			if in[stage-1] != in[stage] {
				return false
			}
		} else {
			// Check if previous assignee same as current
			if in[stage-1] == in[stage] {
				return false
			}
			if stage > 2 {
				if numPlayers == 2 {
					// This check shouldn't be neccessary, but to ensure everything is correct we still do it
					// Also to verify the results of the benchmark
					// Verify that 2 players are always alternating
					if in[stage-2] != in[stage] {
						return false
					}
				} else {
					// Check if assignee 2 stages ago same as current
					if in[stage-2] == in[stage] {
						return false
					}
				}
			}
		}
	}
	return true
}
