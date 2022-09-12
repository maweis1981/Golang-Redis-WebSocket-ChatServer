package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
	"tower/devices"
	. "tower/models"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Handler struct {
	hub    *Hub
	device string
	conn   *websocket.Conn
	Send   chan []byte
}

var connectedUsers = make(map[string]*devices.Plane)

func DeviceWebSocketHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		handleWSError(err, conn)
		return
	}

	deviceToken := mux.Vars(r)["device"]
	log.Println("connected from:", conn.RemoteAddr(), "devices:", deviceToken)

	if uc, exists := connectedUsers[deviceToken]; exists {
		log.Printf("User %s exist, Disconnect first.\n", deviceToken)
		uc.Disconnect(deviceToken)
	}

	u, err := devices.Connect(deviceToken)
	if err != nil {
		log.Printf("Devices Connect Error %n", err)
	}

	connectedUsers[deviceToken] = u

	client := &Handler{
		hub:    hub,
		conn:   conn,
		device: deviceToken,
		Send:   make(chan []byte, 256),
	}
	client.hub.Register <- client
	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	//go client.onConnect(r, conn)
	go client.onChannelMessage(r)
	//go client.onUserMessage(r, conn)
	go client.writePump()
	go client.readPump()

}

func (h *Handler) onDisconnect(r *http.Request, conn *websocket.Conn) chan struct{} {

	closeCh := make(chan struct{})
	username := mux.Vars(r)["device"]
	conn.SetCloseHandler(func(code int, text string) error {
		log.Println("connection closed for devices", username)

		u := connectedUsers[username]
		if err := u.Disconnect(username); err != nil {
			return err
		}
		delete(connectedUsers, username)
		close(closeCh)
		return nil
	})
	return closeCh
}

func (h *Handler) onUserMessage(message []byte) {
	var msg Msg
	log.Printf("On User Message, %s", h.device)
	u := connectedUsers[h.device]

	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("ReadJson Error [%s] \n", err)
	}

	log.Printf("Channel %s, Command %d", msg.Channel, msg.Command)
	switch msg.Command {
	case CommandSubscribe:
		if err := u.Subscribe(msg.Channel); err != nil {
			handleWSError(err, h.conn)
		}
	case CommandUnsubscribe:
		if err := u.Unsubscribe(msg.Channel); err != nil {
			handleWSError(err, h.conn)
		}
	case CommandCall:
		log.Printf("CommandCall %n", msg)
		//check the device status: [IDLE,CALLING,OFFLINE]
		// if idle send call command else response status back.
		h.Send <- []byte("[" + msg.Channel + "] DEVICE OFFLINE " + msg.Content.(string))

	//if err := devices.SendCommand(msg.Channel, msg.Content); err != nil {
	//	handleWSError(err, h.conn)
	//}
	default:
		log.Printf("default running")
	}
}

func (h *Handler) onChannelMessage(r *http.Request) {
	username := mux.Vars(r)["device"]

	log.Printf(username + " OnChannelMessage \n")
	log.Printf("online users length [%d] \n", len(connectedUsers))

	for us := range connectedUsers {
		log.Println(us)
	}
	u := connectedUsers[username]

	for m := range u.MessageChan {
		log.Printf("On Channel Message %s, %s, %s\n", u.Name, m.Payload, m.Channel)
		//msg := Msg{
		//	Content: m.Payload,
		//	Channel: m.Channel,
		//}

		if m.Channel == "general" || m.Channel == "random" {
			h.Send <- []byte(fmt.Sprintf("Broadcast Content= %s, Channel=%s", m.Payload, m.Channel))
		} else {
			for handler := range h.hub.Handlers {
				if handler.device == m.Channel {
					handler.Send <- []byte(fmt.Sprintf("Content= %s, Channel=%s", m.Payload, m.Channel))
				} else {
					log.Printf("Handler Device: %s, Hub Handler Device: %s", m.Channel, handler.device)
				}
			}
		}
	}

}

// HandleUserRegisterEvent will handle the Join event for New socket users
func HandleUserRegisterEvent(hub *Hub, handler *Handler) {
	hub.Handlers[handler] = true
	handleSocketPayloadEvents(handler, SocketEventStruct{
		EventName:    "join",
		EventPayload: handler.device,
		SentTime:     time.Now(),
	})
}

// HandleUserDisconnectEvent will handle the Disconnect event for socket users
func HandleUserDisconnectEvent(hub *Hub, handler *Handler) {
	_, ok := hub.Handlers[handler]
	if ok {
		delete(hub.Handlers, handler)
		close(handler.Send)

		handleSocketPayloadEvents(handler, SocketEventStruct{
			EventName:    "disconnect",
			EventPayload: handler.device,
			SentTime:     time.Now(),
		})
	}
}

func handleSocketPayloadEvents(handler *Handler, socketEventPayload SocketEventStruct) {
	log.Println("Handle Socket Event:[")
	log.Println(socketEventPayload)
	log.Println("]")
	switch socketEventPayload.EventName {
	case "join":
		log.Printf("join")
		BroadcastSocketEventToAllClient(handler.hub, []byte(handler.device+" join in"))
	case "disconnect":
		log.Printf("disconnect")
		BroadcastSocketEventToAllClient(handler.hub, []byte(handler.device+"disconnect"))
	default:
		log.Printf("default ")
	}
}

// BroadcastSocketEventToAllClient will emit the socket events to all socket users
func BroadcastSocketEventToAllClient(hub *Hub, payload []byte) {
	for handler := range hub.Handlers {
		select {
		case handler.Send <- payload:
		default:
			close(handler.Send)
			delete(hub.Handlers, handler)
		}
	}
}

func handleSafeWSError(err error, r *http.Request, conn *websocket.Conn) {
	if conn != nil {
		if err := conn.WriteJSON(Msg{Err: err.Error()}); err != nil {
			log.Printf("Safe Write Json Error %s\n", err)
		}
	} else {
		log.Println("Safe Websocket Connection is nil")
	}
	//onDisconnect(r, conn)
}

func handleWSError(err error, conn *websocket.Conn) {
	if conn != nil {
		if err := conn.WriteJSON(Msg{Err: err.Error()}); err != nil {
			log.Printf("Handle WebSocket Error: %s\n\n", err)
		}
	} else {
		log.Println("Websocket Connection is nil")
	}

}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (h *Handler) readPump() {
	defer func() {
		h.hub.Unregister <- h
		h.conn.Close()
	}()
	h.conn.SetReadLimit(maxMessageSize)
	h.conn.SetReadDeadline(time.Now().Add(pongWait))
	h.conn.SetPongHandler(func(string) error { h.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := h.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		log.Printf("Get Message From %s", message)
		//h.hub.Broadcast <- message
		h.onUserMessage(message)
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (h *Handler) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		h.conn.Close()
	}()
	for {
		select {
		case message, ok := <-h.Send:
			h.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				h.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := h.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(h.Send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-h.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			log.Printf("Write Pump To Client to Keep Connection Valid.")
			h.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := h.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			//h.hub.Broadcast <- []byte(fmt.Sprint("pingpong message %n, %n", h.conn.LocalAddr(), h.conn.RemoteAddr()))
		}
	}
}
