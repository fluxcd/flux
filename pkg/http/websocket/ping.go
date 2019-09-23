package websocket

import (
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer. Needs to be less
	// than the idle timeout on whatever frontend server is proxying the
	// websocket connections (e.g. nginx).
	pongWait = 30 * time.Second

	// Send pings to peer with this period. Must be less than pongWait. The peer
	// must respond with a pong in < pongWait. But it may take writeWait for the
	// pong to be sent. Therefore we want to allow time for that, and a bit of
	// delay/round-trip in case the peer is busy. 1/3 of pongWait seems like a
	// reasonable amount of time to respond to a ping.
	pingPeriod = ((pongWait - writeWait) * 2 / 3)
)

type pingingWebsocket struct {
	pinger    *time.Timer
	readLock  sync.Mutex
	writeLock sync.Mutex
	reader    io.Reader
	conn      *websocket.Conn
}

// Ping adds a periodic ping to a websocket connection.
func Ping(c *websocket.Conn) Websocket {
	p := &pingingWebsocket{conn: c}
	p.conn.SetPongHandler(p.pong)
	p.conn.SetReadDeadline(time.Now().Add(pongWait))
	p.pinger = time.AfterFunc(pingPeriod, p.ping)
	return p
}

func (p *pingingWebsocket) ping() {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	if err := p.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
		p.conn.Close()
		return
	}
	p.pinger.Reset(pingPeriod)
}

func (p *pingingWebsocket) pong(string) error {
	p.conn.SetReadDeadline(time.Now().Add(pongWait))
	return nil
}

func (p *pingingWebsocket) Read(b []byte) (int, error) {
	p.readLock.Lock()
	defer p.readLock.Unlock()

	for p.reader == nil {
		msgType, r, err := p.conn.NextReader()
		if err != nil {
			if IsExpectedWSCloseError(err) {
				return 0, io.EOF
			}
			return 0, err
		}
		if msgType != websocket.BinaryMessage {
			// Ignore non-binary messages for now.
			continue
		}
		p.reader = r
	}

	n, err := p.reader.Read(b)
	if err == io.EOF {
		p.reader = nil
		err = nil
	}
	p.conn.SetReadDeadline(time.Now().Add(pongWait))
	return n, err
}

func (p *pingingWebsocket) Write(b []byte) (int, error) {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	if err := p.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return 0, err
	}
	w, err := p.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	n, err := w.Write(b)
	if err != nil {
		return n, err
	}
	return n, w.Close()
}

func (p *pingingWebsocket) Close() error {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	p.pinger.Stop()
	if err := p.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "ok"), time.Now().Add(writeWait)); err != nil {
		p.conn.Close()
		return err
	}
	return p.conn.Close()
}
