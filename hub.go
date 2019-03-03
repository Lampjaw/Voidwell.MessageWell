package main

type Hub struct {
	subscriptions map[string]map[*Client]bool

	broadcast chan *Message

	register chan *Registration

	unregister chan *Client
}

type Registration struct {
	key string

	client *Client
}

type Message struct {
	key string

	content []byte
}

func newHub() *Hub {
	return &Hub{
		broadcast:     make(chan *Message),
		register:      make(chan *Registration),
		unregister:    make(chan *Client),
		subscriptions: make(map[string]map[*Client]bool),
	}
}

func newRegistration(key string, client *Client) *Registration {
	return &Registration{
		key:    key,
		client: client,
	}
}

func newMessage(key string, content []byte) *Message {
	return &Message{
		key:     key,
		content: content,
	}
}

func (h *Hub) run() {
	for {
		select {
		case registration := <-h.register:
			if h.subscriptions[registration.key] == nil {
				h.subscriptions[registration.key] = make(map[*Client]bool)
			}
			h.subscriptions[registration.key][registration.client] = true
		case client := <-h.unregister:
			for key := range h.subscriptions {
				if _, ok := h.subscriptions[key][client]; ok {
					delete(h.subscriptions[key], client)
					close(client.send)

					if len(h.subscriptions[key]) == 0 {
						delete(h.subscriptions, key)
					}
				}
			}
		case message := <-h.broadcast:
			if h.subscriptions[message.key] != nil {
				for client := range h.subscriptions[message.key] {
					select {
					case client.send <- message.content:
					default:
						close(client.send)
						delete(h.subscriptions[message.key], client)
					}
				}
			}
		}
	}
}
