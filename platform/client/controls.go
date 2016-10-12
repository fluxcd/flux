package client

import (
	"fmt"
	"net/rpc"
	"sync"
)

// ErrInvalidMessage is the error returned when the on-wire message is unexpected.
var ErrInvalidMessage = fmt.Errorf("Invalid Message")

// Message is the unions of Request, Response and arbitrary Value.
type Message struct {
	Request  *rpc.Request
	Response *rpc.Response
	Value    interface{}
}

// JSONWebsocketCodec is golang rpc compatible Server and Client Codec
// that transmits and receives RPC messages over a websocker, as JSON.
type JSONWebsocketCodec struct {
	sync.Mutex
	conn Websocket
	err  chan error
}

// NewJSONWebsocketCodec makes a new JSONWebsocketCodec
func NewJSONWebsocketCodec(conn Websocket) *JSONWebsocketCodec {
	return &JSONWebsocketCodec{
		conn: conn,
		err:  make(chan error, 1),
	}
}

// WaitForReadError blocks until any read on this codec returns an error.
// This is useful to know when the server has disconnected from the client.
func (j *JSONWebsocketCodec) WaitForReadError() error {
	return <-j.err
}

// WriteRequest implements rpc.ClientCodec
func (j *JSONWebsocketCodec) WriteRequest(r *rpc.Request, v interface{}) error {
	j.Lock()
	defer j.Unlock()

	if err := j.conn.WriteJSON(Message{Request: r}); err != nil {
		return err
	}
	return j.conn.WriteJSON(Message{Value: v})
}

// WriteResponse implements rpc.ServerCodec
func (j *JSONWebsocketCodec) WriteResponse(r *rpc.Response, v interface{}) error {
	j.Lock()
	defer j.Unlock()

	if err := j.conn.WriteJSON(Message{Response: r}); err != nil {
		return err
	}
	return j.conn.WriteJSON(Message{Value: v})
}

func (j *JSONWebsocketCodec) readMessage(v interface{}) (*Message, error) {
	m := Message{Value: v}
	if err := j.conn.ReadJSON(&m); err != nil {
		j.err <- err
		close(j.err)
		return nil, err
	}
	return &m, nil
}

// ReadResponseHeader implements rpc.ClientCodec
func (j *JSONWebsocketCodec) ReadResponseHeader(r *rpc.Response) error {
	m, err := j.readMessage(nil)
	if err != nil {
		return err
	}
	if m.Response == nil {
		return ErrInvalidMessage
	}
	*r = *m.Response
	return nil
}

// ReadResponseBody implements rpc.ClientCodec
func (j *JSONWebsocketCodec) ReadResponseBody(v interface{}) error {
	_, err := j.readMessage(v)
	if err != nil {
		return err
	}
	if v == nil {
		return ErrInvalidMessage
	}
	return nil
}

// Close implements rpc.ClientCodec and rpc.ServerCodec
func (j *JSONWebsocketCodec) Close() error {
	return j.conn.Close()
}

// ReadRequestHeader implements rpc.ServerCodec
func (j *JSONWebsocketCodec) ReadRequestHeader(r *rpc.Request) error {
	m, err := j.readMessage(nil)
	if err != nil {
		return err
	}
	if m.Request == nil {
		return ErrInvalidMessage
	}
	*r = *m.Request
	return nil
}

// ReadRequestBody implements rpc.ServerCodec
func (j *JSONWebsocketCodec) ReadRequestBody(v interface{}) error {
	_, err := j.readMessage(v)
	if err != nil {
		return err
	}
	if v == nil {
		return ErrInvalidMessage
	}
	return nil
}
