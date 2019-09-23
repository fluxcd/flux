package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Upgrade upgrades the HTTP server connection to the WebSocket protocol.
func Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (Websocket, error) {
	wsConn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}
	return Ping(wsConn), nil
}
