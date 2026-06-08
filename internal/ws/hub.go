package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	Clients []Client
	mu      sync.RWMutex
}

var HubInstance = &Hub{}

func (h *Hub) AddClient(c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.Clients = append(h.Clients, c)
}

func (h *Hub) RemoveClient(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	targetIndex := -1
	for i, val := range h.Clients {
		if val.ID == id {
			targetIndex = i
			break
		}
	}

	if targetIndex != -1 {
		h.Clients = append(h.Clients[:targetIndex], h.Clients[targetIndex+1:]...)
	}
	
}

func (h *Hub) BroadCast(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	x, err := json.Marshal(msg)
	if err != nil {
		log.Println("Correpted", err)
	}
	for _, client := range h.Clients {
		client.Conn.WriteMessage(websocket.TextMessage, x)

	}
}
