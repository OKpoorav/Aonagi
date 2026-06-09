package ws

import (
	"aonagi/internal/models"
	"aonagi/internal/trips"
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	clients            map[string]*Client
	mu                 sync.RWMutex
	OnMessageBroadcast func(msg models.Message)
}

type Envelope struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

var HubInstance = &Hub{
	clients: make(map[string]*Client),
}

func (h *Hub) AddClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[c.ID] = c
}

func (h *Hub) RemoveClient(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, id)
}

func (h *Hub) BroadCast(msg models.Message) {
	// Store the message in the room message history
	room := trips.GetOrCreateRoom(msg.GroupID)
	room.AddMessage(msg)

	h.mu.RLock()
	defer h.mu.RUnlock()

	envelope := Envelope{
		Type:    "chat",
		Payload: msg,
	}

	x, err := json.Marshal(envelope)
	if err != nil {
		log.Println("Corrupted message:", err)
		return
	}

	for _, client := range h.clients {
		if client.RoomID == msg.GroupID {
			err := client.Conn.WriteMessage(websocket.TextMessage, x)
			if err != nil {
				log.Printf("Error writing to client %s: %s\n", client.ID, err)
			}
		}
	}

	// Trigger AI observer callback if set
	if h.OnMessageBroadcast != nil {
		go h.OnMessageBroadcast(msg)
	}
}

func (h *Hub) BroadCastTripState(roomID string, state trips.TripState) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	envelope := Envelope{
		Type:    "trip_state",
		Payload: state,
	}

	x, err := json.Marshal(envelope)
	if err != nil {
		log.Println("Error marshaling trip state:", err)
		return
	}

	for _, client := range h.clients {
		if client.RoomID == roomID {
			err := client.Conn.WriteMessage(websocket.TextMessage, x)
			if err != nil {
				log.Printf("Error writing trip state to client %s: %s\n", client.ID, err)
			}
		}
	}
}

func (h *Hub) BroadCastPolls(roomID string, polls []*trips.Poll) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	envelope := Envelope{
		Type:    "polls",
		Payload: polls,
	}

	x, err := json.Marshal(envelope)
	if err != nil {
		log.Println("Error marshaling polls:", err)
		return
	}

	for _, client := range h.clients {
		if client.RoomID == roomID {
			err := client.Conn.WriteMessage(websocket.TextMessage, x)
			if err != nil {
				log.Printf("Error writing polls to client %s: %s\n", client.ID, err)
			}
		}
	}
}

func (h *Hub) BroadCastDecisions(roomID string, decisions map[string]trips.Decision) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	envelope := Envelope{
		Type:    "decisions",
		Payload: decisions,
	}

	x, err := json.Marshal(envelope)
	if err != nil {
		log.Println("Error marshaling decisions:", err)
		return
	}

	for _, client := range h.clients {
		if client.RoomID == roomID {
			err := client.Conn.WriteMessage(websocket.TextMessage, x)
			if err != nil {
				log.Printf("Error writing decisions to client %s: %s\n", client.ID, err)
			}
		}
	}
}

func (h *Hub) BroadCastSuggestions(roomID string, suggestions []trips.PollSuggestion) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	envelope := Envelope{
		Type:    "suggestions",
		Payload: suggestions,
	}

	x, err := json.Marshal(envelope)
	if err != nil {
		log.Println("Error marshaling suggestions:", err)
		return
	}

	for _, client := range h.clients {
		if client.RoomID == roomID {
			err := client.Conn.WriteMessage(websocket.TextMessage, x)
			if err != nil {
				log.Printf("Error writing suggestions to client %s: %s\n", client.ID, err)
			}
		}
	}
}
