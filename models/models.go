package models

import "time"

type Msg struct {
	Content           string             `json:"content,omitempty"`
	Channel           string             `json:"channel,omitempty"`
	Command           int                `json:"command,omitempty"`
	Err               string             `json:"err,omitempty"`
	MessagePayload    *MessagePayload    `json:"messagePayload,omitempty"`
	SocketEventStruct *SocketEventStruct `json:"socketEventStruct,omitempty"`
}

const (
	CommandSubscribe = iota
	CommandUnsubscribe
	CommandChat
)

type MessagePayload struct {
	Event string `json:"event,omitempty"`
}

type SocketEventStruct struct {
	EventName    string      `json:"event,omitempty"`
	EventPayload interface{} `json:"payload,omitempty"`
	SentTime     time.Time   `json:"sent_time,omitempty"`
}
