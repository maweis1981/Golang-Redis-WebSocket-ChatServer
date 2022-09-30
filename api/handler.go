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
	send                chan []byte
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
		log.Println("defer connection closed")
		h.webSocketConnection.Close()
	}()
	defer unRegisterAndCloseConnection(h.webSocketConnection)

	for {
		log.Println("ReadPump Working...................")
		_, payload, err := h.webSocketConnection.ReadMessage()

		//log.Printf("payload Size :\t %v \n", len(payload))
		//log.Printf("payload Body :\t %v \n", payload)
		//log.Printf("Error : [%s]", err)

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
			log.Printf("Read Pump Error ===: #{err}")
			break
		}
		payload = bytes.TrimSpace(bytes.Replace(payload, newline, space, -1))
		log.Println("Read Message : " + string(payload))

		//h.webSocketConnection.WriteJSON("connection normal.")

		log.Println("Read User Connection Name " + h.userName)
		log.Println("Read User Connection Remote " + h.webSocketConnection.RemoteAddr().String())
		log.Println("Read User Connection Local " + h.webSocketConnection.LocalAddr().String())

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
		case message, ok := <-h.send:
			log.Println("writePump Message Now")
			h.webSocketConnection.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				h.webSocketConnection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := h.webSocketConnection.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			log.Println("writePump Ticker Now")
			h.webSocketConnection.SetWriteDeadline(time.Now().Add(writeWait))
			if err := h.webSocketConnection.WriteMessage(websocket.PingMessage, nil); err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					log.Println("websocket.ticker: client side closed socket " + h.userName)
				} else {
					log.Println("websocket.ticker: closing " + h.userName + ", " + err.Error())
				}
				return
			} else {
				log.Printf("Write Message %d", websocket.PingMessage)
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
		send:                make(chan []byte, 256),
	}

	setSocketPayloadReadConfig(h.webSocketConnection)

	go h.readPump()
	go h.writePump()

	err = onConnect(r, conn)
	if err != nil {
		handleWSError(err, conn)
		return
	}

	//closeCh := onDisconnect(r, conn)
	h.onChannelMessage()
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
		uc.Disconnect()
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
	log.Println("connection closed for devices", username)

	conn.SetCloseHandler(func(code int, text string) error {
		u := connectedUsers[username]
		if err := u.Disconnect(); err != nil {
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
	log.Println(msg.Command)
	log.Println(msg.MessagePayload.Event)
	log.Println(msg.SocketEventStruct.EventName)
	log.Println(msg.SocketEventStruct.EventPayload)

	u := connectedUsers[h.userName]
	switch msg.Command {
	case CommandSubscribe:

		if err := u.Subscribe(msg.Channel); err != nil {
			handleWSError(err, u.WebSocketConnection)
		}
	case CommandUnsubscribe:
		if err := u.Unsubscribe(msg.Channel); err != nil {
			handleWSError(err, u.WebSocketConnection)
		}
	case CommandChat:
		m, _ := json.Marshal(msg)
		if err := devices.SendCommand(msg.Channel, string(m)); err != nil {
			handleWSError(err, u.WebSocketConnection)
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
			log.Println("User Connection Name " + u.Name + " == " + h.userName)
			log.Println("User Connection Remote " + u.WebSocketConnection.RemoteAddr().String() + " == " + h.webSocketConnection.RemoteAddr().String())
			log.Println("User Connection Local " + u.WebSocketConnection.LocalAddr().String() + " == " + h.webSocketConnection.LocalAddr().String())

			h.send <- []byte(msg.Content)
			//if err := h.webSocketConnection.WriteJSON(msg); err != nil {
			//if err := h.webSocketConnection.WriteJSON(msg); err != nil {
			//	log.Printf("Client Disconnect Error [%s] ", err)
			//	u.Disconnect()
			//	delete(connectedUsers, h.userName)
			//}
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
