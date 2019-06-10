
package main

import (
	"github.com/google/uuid"
)

type Message struct {
	sender uuid.UUID
	bytes []byte
	fromServer bool
}

type Action struct {
	Source string `json:"source"`
	Destination string `json:"destination"`
	Method string `json:"method"`
	Parameter string `json:"parameter"`
}