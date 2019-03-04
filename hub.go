package main

import (
	"encoding/json"
	"reflect"
)

type Hub struct {
	clients       map[*Client]map[string]bool
	subscriptions map[string]map[*Client]bool
	broadcast     chan *BroadcastMessage
	register      chan *Client
	unregister    chan *Client
	unsubscribe   chan *SubscriptionChange
	subscribe     chan *SubscriptionChange
}

type SubscriptionChange struct {
	keys   []string
	client *Client
}

type BroadcastMessage struct {
	Event   string      `json:"event"`
	Message interface{} `json:"message"`
}

func newHub() *Hub {
	return &Hub{
		broadcast:     make(chan *BroadcastMessage),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		unsubscribe:   make(chan *SubscriptionChange),
		subscribe:     make(chan *SubscriptionChange),
		clients:       make(map[*Client]map[string]bool),
		subscriptions: make(map[string]map[*Client]bool),
	}
}

func newSubscriptionChange(client *Client, keys []string) *SubscriptionChange {
	return &SubscriptionChange{
		client: client,
		keys:   keys,
	}
}

func newBroadcastMessage(key string, content []byte) *BroadcastMessage {
	var message interface{}
	json.Unmarshal(content, &message)
	return &BroadcastMessage{
		Event:   key,
		Message: message,
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = make(map[string]bool)
		case client := <-h.unregister:
			h.removeClient(client)
		case subscriptionChange := <-h.subscribe:
			h.addSubscriptions(subscriptionChange.client, subscriptionChange.keys)
		case subscriptionChange := <-h.unsubscribe:
			h.removeSubscriptions(subscriptionChange.client, subscriptionChange.keys)
		case broadcast := <-h.broadcast:
			if _, ok := h.subscriptions[broadcast.Event]; ok {
				message, _ := json.Marshal(broadcast)
				for client := range h.subscriptions[broadcast.Event] {
					select {
					case client.send <- message:
					default:
						h.removeClient(client)
					}
				}
			}
		}
	}
}

func (h *Hub) getSubscriptions(c *Client) []string {
	if subKeys, ok := h.clients[c]; ok {
		keys := reflect.ValueOf(subKeys).MapKeys()
		strkeys := make([]string, len(keys))
		for i := 0; i < len(keys); i++ {
			strkeys[i] = keys[i].String()
		}
		return strkeys
	}

	return nil
}

func (h *Hub) addSubscriptions(c *Client, subKeys []string) {
	if _, ok := h.clients[c]; ok {
		for _, subKey := range subKeys {
			h.clients[c][subKey] = true
			if _, ok := h.subscriptions[subKey]; !ok {
				h.subscriptions[subKey] = make(map[*Client]bool)
			}
			h.subscriptions[subKey][c] = true
		}
	}
}

func (h *Hub) removeClient(c *Client) {
	if _, ok := h.clients[c]; ok {
		subKeys := h.getSubscriptions(c)
		h.removeSubscriptions(c, subKeys)

		delete(h.clients, c)
		close(c.send)
	}
}

func (h *Hub) removeSubscriptions(c *Client, subKeys []string) {
	if _, ok := h.clients[c]; ok {
		for _, key := range subKeys {
			if _, ok := h.clients[c][key]; ok {
				delete(h.clients[c], key)
			}

			if _, ok := h.subscriptions[key]; ok {
				if _, ok := h.subscriptions[key][c]; ok {
					delete(h.subscriptions[key], c)

					if len(h.subscriptions[key]) == 0 {
						delete(h.subscriptions, key)
					}
				}
			}
		}
	}
}
