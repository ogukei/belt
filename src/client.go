
package main

import (
	"log"
	"net/http"
	"time"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second
	pongWait = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{}

type Channel struct {
	hub *Hub
	conn *websocket.Conn
	send chan []byte
	id uuid.UUID
	server bool
}

func (c *Channel) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(0)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, bytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message := Message { sender: c.id, bytes: bytes, fromServer: c.server }
		c.hub.dispatch <- message
	}
}

func (c *Channel) writePump() {
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

func serveClientWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	uuid, err := uuid.NewRandom()
	if err != nil {
		log.Println(err)
		return
	}
	channel := &Channel{ hub: hub, 
		conn: conn, 
		send: make(chan []byte, 256),
		id: uuid,
		server: false }
	channel.hub.register <- channel

	go channel.writePump()
	go channel.readPump()
}

func serveServerWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	uuid, err := uuid.NewRandom()
	if err != nil {
		log.Println(err)
		return
	}
	channel := &Channel{ hub: hub, 
		conn: conn, 
		send: make(chan []byte, 256),
		id: uuid,
		server: true }
	channel.hub.register <- channel

	go channel.writePump()
	go channel.readPump()
}