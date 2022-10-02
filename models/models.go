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
	CommandCall
)

type MessagePayload struct {
	Event string `json:"event,omitempty"`
}

type SocketEventStruct struct {
	EventName    string      `json:"event"`
	EventPayload interface{} `json:"payload"`
	SentTime     time.Time   `json:"sent_time"`
}

/**
ON Signal Server
Enroll - DeviceToken, Add to online-members-list
TurnOn Personal Channel - DeviceToken As ChannelName
TurnOff Personal Channel - DeviceToken As ChannelName
SetStatus - DeviceToken, Status:IDLE(Default), BUSY, DoNotDisturb.
CreateRoom - Room Master, Subscribe Room Channel
Invite - Send Message to Callee
Accept -

On Client Side

Call-Out - Generate Room ID and create Channel, Send Message to Callee,
	Callee Get Message, Play Callee Audio/Video Ringtone.
Call-In - Play Audio Ringtone, Video Ringtone, If Accept, the Channel subscriber will
	be joined in the Audio/Video ChatRoom.

Command:
	Command

struct {
	DeviceToken
	ChannelName
	Command
	Status
	RoomID
	RoomMaster
	Caller
	Callee
	Content
	Ringtone
}

URL : /ws/deviceToken
enroll: {}
TurnOn: {"command":0, deviceToken}
TurnOff: {"command": 1, deviceToken}
SetStatus: {"command": 3, deviceToken, "IDLE"}
SetStatus: {"command": 3, deviceToken, "BUSY"}
SetStatus: {"command": 3, deviceToken, "DoNotDisturb"}
CreateRoom: {"command": 4, YouMeRoomId}
Invite: {"command": 5, "callee_device_token", "youMeRoomId As Channel Name"}
Accept: {"command": 6, ""}


API: /api/users


*/
