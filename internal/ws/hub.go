package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	clients map[string]*Client
	mu      sync.RWMutex
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

func (h *Hub) BroadCast(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	x, err := json.Marshal(msg)
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
}
