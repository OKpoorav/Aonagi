package ws

import (
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

	client := &Client{
		ID:       generateUUID(),
		Username: username,
		RoomID:   roomID,
		Conn:     conn,
	}
	HubInstance.AddClient(client)

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
		var msg Message
		err = json.Unmarshal(p, &msg)
		if err != nil {
			log.Printf("JSON unmarshal error: %s\n", err)
			continue
		}
		
		// Ensure system security and correct identity information
		msg.GroupID = client.RoomID
		msg.SenderID = client.ID
		msg.Name = client.Username
		msg.TimeStamp = time.Now()

		HubInstance.BroadCast(msg)
	}
}
