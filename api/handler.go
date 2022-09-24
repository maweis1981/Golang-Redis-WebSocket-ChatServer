package api

import (
	"bytes"
	"encoding/json"
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
	//connectedUsers map[string]*devices.Plane
	userName            string          `json:"device"`
	webSocketConnection *websocket.Conn `json:"web_socket_connection"`
	//send                chan string
}

var connectedUsers = make(map[string]*devices.Plane)

func setSocketPayloadReadConfig(c *websocket.Conn) {
	c.SetReadLimit(maxMessageSize)
	c.SetReadDeadline(time.Now().Add(pongWait))
	c.SetPongHandler(func(string) error { c.SetReadDeadline(time.Now().Add(pongWait)); return nil })
}

func unRegisterAndCloseConnection(c *websocket.Conn) {
	c.Close()
}

func (h *Handler) readPump() {
	log.Println("readPump Running.")
	defer func() {
		h.webSocketConnection.Close()
	}()
	defer unRegisterAndCloseConnection(h.webSocketConnection)
	setSocketPayloadReadConfig(h.webSocketConnection)

	for {
		_, payload, err := h.webSocketConnection.ReadMessage()

		log.Printf("payload Size :\t %v \n", len(payload))
		log.Printf("payload Body :\t %v \n", payload)
		log.Printf("Error : [%n]", err)

		//if len(payload) > 0 {
		//decoder := json.NewDecoder(bytes.NewReader(payload))
		//decoderErr := decoder.Decode(&socketEventPayload)

		//if decoderErr != nil {
		//	log.Printf("Decode error: %v", decoderErr)
		//	break
		//}

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error ===: %v", err)
			}
			break
		}
		payload = bytes.TrimSpace(bytes.Replace(payload, newline, space, -1))
		log.Println("Read Message %n", payload)

		h.executeCommand(payload)

		/**
		On User Message From Here.
		It needs to unmarshal json data and dispatch data to others.
		*/

		//handleSocketPayloadEvents(c, socketEventPayload)
		//} else {
		//	log.Println("Payload size equal zero")
		//	break
		//}
	}
}

func (h *Handler) writePump() {
	log.Println("writePump Running.")
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		h.webSocketConnection.Close()
	}()
	for {
		select {
		case <-ticker.C:
			log.Println("writePump Ticker Now")
			h.webSocketConnection.SetWriteDeadline(time.Now().Add(writeWait))
			if err := h.webSocketConnection.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("Write Pump Error [%n]", err)
				return
			} else {
				log.Println("Write Message %n", websocket.PingMessage)
			}
		}
	}
}

func DeviceWebSocketHandler(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		handleWSError(err, conn)
		return
	}

	username := mux.Vars(r)["device"]
	h := &Handler{
		userName:            username,
		webSocketConnection: conn,
	}

	go h.readPump()
	go h.writePump()

	err = onConnect(r, conn)
	if err != nil {
		handleWSError(err, conn)
		return
	}

	//closeCh := onDisconnect(r, conn)
	go h.onChannelMessage()
	//
	//loop:
	//	for {
	//		select {
	//		case <-closeCh:
	//			break loop
	//		default:
	//			onUserMessage(conn, r)
	//		}
	//	}
}

/**
Device Connect
*/
func onConnect(r *http.Request, conn *websocket.Conn) error {
	username := mux.Vars(r)["device"]
	log.Println("connected from:", conn.RemoteAddr(), "devices:", username)

	// if the same user connect, the old connection will be disconnect.
	if uc, exists := connectedUsers[username]; exists {
		log.Printf("User %s exist, Disconnect first.\n", username)
		uc.Disconnect(username)
	}

	u, err := devices.Connect(username, conn)
	if err != nil {
		return err
	}

	connectedUsers[username] = u
	return nil
}

func onDisconnect(r *http.Request, conn *websocket.Conn) chan struct{} {

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

func (h *Handler) executeCommand(message []byte) {
	var msg Msg
	err := json.Unmarshal(message, &msg)
	if err != nil {
		log.Println(err)
	}
	log.Println(msg)

	u := connectedUsers[h.userName]
	switch msg.Command {
	case CommandSubscribe:

		if err := u.Subscribe(msg.Channel); err != nil {
			handleWSError(err, h.webSocketConnection)
		}
	case CommandUnsubscribe:
		if err := u.Unsubscribe(msg.Channel); err != nil {
			handleWSError(err, h.webSocketConnection)
		}
	case CommandChat:
		if err := devices.SendCommand(msg.Channel, msg.Content); err != nil {
			handleWSError(err, h.webSocketConnection)
		}
	}
}

func onUserMessage(conn *websocket.Conn, r *http.Request) {
	var msg Msg

	username := mux.Vars(r)["device"]
	u := connectedUsers[username]

	if err := conn.ReadJSON(&msg); err != nil {
		log.Printf("ReadJson Error [%s] \n", err)
		handleSafeWSError(err, r, conn)
		return
	}

	switch msg.Command {
	case CommandSubscribe:
		if err := u.Subscribe(msg.Channel); err != nil {
			handleWSError(err, conn)
		}
	case CommandUnsubscribe:
		if err := u.Unsubscribe(msg.Channel); err != nil {
			handleWSError(err, conn)
		}
	case CommandChat:
		if err := devices.SendCommand(msg.Channel, msg.Content); err != nil {
			handleWSError(err, conn)
		}
	}
}

func (h *Handler) onChannelMessage() {
	// ticker := time.NewTicker(pingPeriod)
	// defer func() {
	//	ticker.Stop()
	//	h.webSocketConnection.Close()
	// }()

	log.Printf(h.userName + " OnChannelMessage \n")
	log.Printf("online users length [%d] \n", len(connectedUsers))

	for us := range connectedUsers {
		log.Println(us)
	}

	u := connectedUsers[h.userName]

	go func() {
		for m := range u.MessageChan {
			log.Printf("On Channel Message %s, %s, %s\n", u.Name, m.Payload, m.Channel)

			//conn.WriteMessage(websocket.TextMessage, )

			msg := Msg{
				Content: m.Payload,
				Channel: m.Channel,
			}
			//conn.WriteJSON(msg)
			//
			if err := h.webSocketConnection.WriteJSON(msg); err != nil {
				log.Printf("Client Disconnect Error [%s] ", err)
				//	delete(connectedUsers, username)
			}
		}
	}()
}

func handleSafeWSError(err error, r *http.Request, conn *websocket.Conn) {

	if conn != nil {
		if err := conn.WriteJSON(Msg{Err: err.Error()}); err != nil {
			log.Printf("Safe Write Json Error %s\n", err)
		}
	} else {
		log.Println("Safe Websocket Connection is nil")
	}

	onDisconnect(r, conn)
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
