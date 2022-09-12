package api

type Hub struct {
	Handlers   map[*Handler]bool
	Broadcast  chan []byte
	Whisper    chan []byte
	Register   chan *Handler
	Unregister chan *Handler
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		Register:   make(chan *Handler),
		Unregister: make(chan *Handler),
		Handlers:   make(map[*Handler]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Handlers[client] = true
			HandleUserRegisterEvent(h, client)
		case client := <-h.Unregister:
			HandleUserDisconnectEvent(h, client)
			if _, ok := h.Handlers[client]; ok {
				delete(h.Handlers, client)
				close(client.Send)
			}
		case message := <-h.Broadcast:
			for client := range h.Handlers {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.Handlers, client)
				}
			}
		}
	}
}
