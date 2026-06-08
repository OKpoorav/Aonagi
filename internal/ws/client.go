package ws

import (
	"github.com/gorilla/websocket"
)

type Client struct {
	ID       string
	Username string
	RoomID   string
	Conn     *websocket.Conn
}
