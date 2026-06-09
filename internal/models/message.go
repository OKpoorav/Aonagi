package models

import "time"

type Message struct {
	MessageID string
	GroupID   string
	SenderID  string
	Name      string
	Body      string
	TimeStamp time.Time
}
