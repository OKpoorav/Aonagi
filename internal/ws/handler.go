package ws

import (
	"aonagi/internal/models"
	"aonagi/internal/trips"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ClientMessage struct {
	MessageID string          `json:"MessageID"`
	GroupID   string          `json:"GroupID"`
	SenderID  string          `json:"SenderID"`
	Name      string          `json:"Name"`
	Body      string          `json:"Body"`
	TimeStamp time.Time       `json:"TimeStamp"`

	Action   string          `json:"Action"`
	PollData json.RawMessage `json:"PollData"`
}

func generateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func Handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "general"
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		username = "Anonymous-" + generateUUID()[:6]
	}

	userID := r.URL.Query().Get("userid")
	if userID == "" {
		userID = generateUUID()
	}

	client := &Client{
		ID:       userID,
		Username: username,
		RoomID:   roomID,
		Conn:     conn,
	}
	HubInstance.AddClient(client)

	// Synchronize full room history, state, and polls to the client sequentially
	room := trips.GetOrCreateRoom(roomID)

	// 1. History
	historyEnv := Envelope{
		Type:    "history",
		Payload: room.GetMessages(),
	}
	if x, err := json.Marshal(historyEnv); err == nil {
		conn.WriteMessage(websocket.TextMessage, x)
	}

	// 2. Trip State
	stateEnv := Envelope{
		Type:    "trip_state",
		Payload: room.GetTripState(),
	}
	if x, err := json.Marshal(stateEnv); err == nil {
		conn.WriteMessage(websocket.TextMessage, x)
	}

	// 3. Polls
	pollsEnv := Envelope{
		Type:    "polls",
		Payload: room.GetPolls(),
	}
	if x, err := json.Marshal(pollsEnv); err == nil {
		conn.WriteMessage(websocket.TextMessage, x)
	}

	// 4. Decisions
	decisionsEnv := Envelope{
		Type:    "decisions",
		Payload: room.GetDecisions(),
	}
	if x, err := json.Marshal(decisionsEnv); err == nil {
		conn.WriteMessage(websocket.TextMessage, x)
	}

	// 5. Suggestions
	suggestionsEnv := Envelope{
		Type:    "suggestions",
		Payload: room.GetSuggestions(),
	}
	if x, err := json.Marshal(suggestionsEnv); err == nil {
		conn.WriteMessage(websocket.TextMessage, x)
	}

	defer func() {
		HubInstance.RemoveClient(client.ID)
		conn.Close()
	}()

	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Client %s disconnected: %v\n", client.ID, err)
			return
		}
		var clientMsg ClientMessage
		err = json.Unmarshal(p, &clientMsg)
		if err != nil {
			log.Printf("JSON unmarshal error: %s\n", err)
			continue
		}

		if clientMsg.Action != "" {
			handlePollAction(client, clientMsg)
		} else {
			msg := models.Message{
				MessageID: clientMsg.MessageID,
				GroupID:   client.RoomID,
				SenderID:  client.ID,
				Name:      client.Username,
				Body:      clientMsg.Body,
				TimeStamp: time.Now(),
			}
			HubInstance.BroadCast(msg)
		}
	}
}

func handlePollAction(client *Client, msg ClientMessage) {
	room := trips.GetOrCreateRoom(client.RoomID)

	switch msg.Action {
	case "create_poll":
		var payload struct {
			Question string   `json:"question"`
			Category string   `json:"category"`
			Options  []string `json:"options"`
		}
		if err := json.Unmarshal(msg.PollData, &payload); err != nil {
			log.Println("Invalid create_poll payload:", err)
			return
		}

		_, err := room.CreatePoll(payload.Question, payload.Category, payload.Options, client.ID)
		if err != nil {
			log.Println("Create poll error:", err)
			return
		}

		HubInstance.BroadCastPolls(room.ID, room.GetPolls())
		HubInstance.BroadCastSuggestions(room.ID, room.GetSuggestions())

	case "vote_poll":
		var payload struct {
			PollID string `json:"pollId"`
			Option string `json:"option"`
		}
		if err := json.Unmarshal(msg.PollData, &payload); err != nil {
			log.Println("Invalid vote_poll payload:", err)
			return
		}

		err := room.VotePoll(payload.PollID, client.ID, payload.Option)
		if err != nil {
			log.Println("Vote poll error:", err)
			return
		}

		HubInstance.BroadCastPolls(room.ID, room.GetPolls())

	case "close_poll":
		var payload struct {
			PollID string `json:"pollId"`
		}
		if err := json.Unmarshal(msg.PollData, &payload); err != nil {
			log.Println("Invalid close_poll payload:", err)
			return
		}

		_, err := room.ClosePoll(payload.PollID, client.ID)
		if err != nil {
			log.Println("Close poll error:", err)
			return
		}

		HubInstance.BroadCastPolls(room.ID, room.GetPolls())
		HubInstance.BroadCastTripState(room.ID, room.GetTripState())
		HubInstance.BroadCastDecisions(room.ID, room.GetDecisions())
		HubInstance.BroadCastSuggestions(room.ID, room.GetSuggestions())
	}
}
