
package main

import (
	"encoding/json"
	"log"
)

type Hub struct {
	channels map[*Channel]bool
	dispatch chan Message
	register chan *Channel
	unregister chan *Channel
}

func newHub() *Hub {
	return &Hub {
		dispatch: make(chan Message),
		register: make(chan *Channel),
		unregister: make(chan *Channel),
		channels: make(map[*Channel]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case channel := <-h.register:
			h.channels[channel] = true
		case channel := <-h.unregister:
			log.Printf("channel %v disconnected (server: %v)", channel.id, channel.server)
			if _, ok := h.channels[channel]; ok {
				delete(h.channels, channel)
				close(channel.send)
				// notify the server that the channel has closed
				action := Action { Method: "close", 
					Source: channel.id.String(), 
					Parameter: "remote disconnected" }
				if data, err := json.Marshal(action); err == nil {
					for channel := range h.channels {
						if channel.server {
							select {
							case channel.send <- data:
							default:
							}
						}
					}
				}
			}
		case message := <-h.dispatch:
			var action Action 
			if err := json.Unmarshal(message.bytes, &action); err != nil {
				log.Printf("json decode error: %v", err)
				break
			}
			for channel := range h.channels {
				if message.sender == channel.id {
					continue
				}
				if message.fromServer && action.Destination == channel.id.String() {
					// hide Source
					action = Action { Destination: channel.id.String(),
						Method: action.Method, Parameter: action.Parameter }
					var data []byte
					var err error
					if data, err = json.Marshal(action); err != nil {
						log.Printf("json Marshal error: %v", err)
						continue
					}
					select {
					case channel.send <- data:
					default:
						close(channel.send)
						delete(h.channels, channel)
					}
				}
				if !message.fromServer && channel.server {
					// pass method and parameter
					action = Action { Source: message.sender.String(),
						Method: action.Method, Parameter: action.Parameter }
					var data []byte
					var err error
					if data, err = json.Marshal(action); err != nil {
						log.Printf("json Marshal error: %v", err)
						continue
					}
					select {
					case channel.send <- data:
					default:
						close(channel.send)
						delete(h.channels, channel)
					}
				}
			}
		}
	}
}