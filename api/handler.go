package api

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"tower/devices"
	. "tower/models"
)

var upgrader websocket.Upgrader

var connectedUsers = make(map[string]*devices.Plane)

func DeviceWebSocketHandler(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		handleWSError(err, conn)
		return
	}
	defer conn.Close()

	err = onConnect(r, conn)
	if err != nil {
		handleWSError(err, conn)
		return
	}

	closeCh := onDisconnect(r, conn)
	onChannelMessage(conn, r)

loop:
	for {
		select {
		case <-closeCh:
			break loop
		default:
			onUserMessage(conn, r)
		}
	}
}

func onConnect(r *http.Request, conn *websocket.Conn) error {
	username := mux.Vars(r)["device"]
	log.Println("connected from:", conn.RemoteAddr(), "devices:", username)

	u, err := devices.Connect(username)
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

func onUserMessage(conn *websocket.Conn, r *http.Request) {
	var msg Msg

	username := mux.Vars(r)["device"]
	u := connectedUsers[username]

	if err := conn.ReadJSON(&msg); err != nil {
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

func onChannelMessage(conn *websocket.Conn, r *http.Request) {
	username := mux.Vars(r)["device"]

	log.Printf(username + " OnChannelMessage \n")
	log.Printf("online users length [%d] \n", len(connectedUsers))

	for us := range connectedUsers {
		log.Println(us)
	}

	u := connectedUsers[username]

	go func() {
		for m := range u.MessageChan {
			log.Printf("On Channel Message %s, %s, %s\n", u.Name, m.Payload, m.Channel)

			msg := Msg{
				Content: m.Payload,
				Channel: m.Channel,
			}

			if err := conn.WriteJSON(msg); err != nil {
				log.Println(err)
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
