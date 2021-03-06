package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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

type Client struct {
	hub *Hub

	conn *websocket.Conn

	send chan []byte
}

type ReceiveMessage struct {
	Action string   `json:"action"`
	Events []string `json:"events"`
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, msgBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws receive error: %v", err)
			}
			break
		}

		var message ReceiveMessage
		err2 := json.Unmarshal(msgBytes, &message)
		if err2 != nil {
			c.send <- []byte(fmt.Sprintf("SUBSCRIPTION ERROR: %v", err2))
			continue
		}

		switch message.Action {
		case "subscribe":
			c.addSubscriptions(message.Events)
		case "unsubscribe":
			c.removeSubscriptions(message.Events)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) addSubscriptions(subKeys []string) {
	if subKeys != nil {
		c.hub.subscribe <- newSubscriptionChange(c, subKeys)
	}
}

func (c *Client) removeSubscriptions(subKeys []string) {
	if subKeys != nil {
		c.hub.unsubscribe <- newSubscriptionChange(c, subKeys)
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(req *http.Request) bool {
		return true
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}

	vars := mux.Vars(r)
	key := vars["key"]

	client.hub.register <- client

	if key != "" {
		subKeys := make([]string, 1)
		subKeys[0] = key
		client.addSubscriptions(subKeys)
	}

	go client.writePump()
	go client.readPump()
}

func publishWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		return
	}

	vars := mux.Vars(r)
	key := vars["key"]

	hub.broadcast <- newBroadcastMessage(key, bodyBytes)
}
