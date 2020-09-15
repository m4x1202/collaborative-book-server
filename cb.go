package cb

const (
	AWSRegion          = "eu-central-1"
	APIGatewayEndpoint = "r8sc9tucc2.execute-api.eu-central-1.amazonaws.com/dev"
	DynamoDBTable      = "collaborative-book-connections"
	DefaultRoomName    = "unknown"
)

type RoomState int

const (
	Lobby RoomState = iota
	WriteStories
	ShowStories
)

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

/// DynamoDB item

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
