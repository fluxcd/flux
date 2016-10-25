package websocket

import (
	"io"

	"github.com/gorilla/websocket"
)

// Websocket exposes the bits of *websocket.Conn we actually use. Note
// that we are emulating an `io.ReadWriter`. This is to be able
// to support RPC codecs, which operate on byte streams.
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
