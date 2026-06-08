package ws

import (
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

func Handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := Client{
		ID:       time.Now().String(),
		Username: "Temp",
		RoomID:   "Temp",
		Conn:     conn,
	}
	HubInstance.AddClient(client)

	defer conn.Close()
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			HubInstance.RemoveClient(client.ID)
			return
		}
		var msg Message
		err = json.Unmarshal(p, &msg)
		if err != nil {
			fmt.Printf("TCP accept error: %s\n", err)
			continue
		}
		HubInstance.BroadCast(msg)

	}
}
