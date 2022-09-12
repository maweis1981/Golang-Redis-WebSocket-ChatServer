package models

import "time"

type Msg struct {
	Content interface{} `json:"content,omitempty"`
	Channel string      `json:"channel,omitempty"`
	Command int         `json:"command,omitempty"`
	Err     string      `json:"err,omitempty"`
}

const (
	CommandSubscribe = iota
	CommandUnsubscribe
	CommandCall
)

type MessagePayload struct {
}

type SocketEventStruct struct {
	EventName    string      `json:"event"`
	EventPayload interface{} `json:"payload"`
	SentTime     time.Time   `json:"sent_time"`
}
