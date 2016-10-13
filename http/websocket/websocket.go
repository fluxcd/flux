package websocket

import (
	"io"

	"github.com/gorilla/websocket"
)

// Websocket exposes the bits of *websocket.Conn we actually use.
type Websocket interface {
	io.Reader
	io.Writer
	Close() error
}

// IsExpectedWSCloseError returns boolean indicating whether the error is a
// clean disconnection.
func IsExpectedWSCloseError(err error) bool {
	return err == io.EOF || err == io.ErrClosedPipe || websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseAbnormalClosure,
	)
}
