package models

type Msg struct {
	Content string `json:"content,omitempty"`
	Channel string `json:"channel,omitempty"`
	Command int    `json:"command,omitempty"`
	Err     string `json:"err,omitempty"`
}

const (
	CommandSubscribe = iota
	CommandUnsubscribe
	CommandChat
)

type MessagePayload struct {
}
