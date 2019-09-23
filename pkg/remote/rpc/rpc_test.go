package rpc

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/remote"
)

func pipes() (io.ReadWriteCloser, io.ReadWriteCloser) {
	type end struct {
		io.Reader
		io.WriteCloser
	}

	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	return end{clientReader, clientWriter}, end{serverReader, serverWriter}
}

func TestRPC(t *testing.T) {
	wrap := func(mock api.Server) api.Server {
		clientConn, serverConn := pipes()

		server, err := NewServer(mock, 10*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		go server.ServeConn(serverConn)
		return NewClientV11(clientConn)
	}
	remote.ServerTestBattery(t, wrap)
}

// ---

type poorReader struct{}

func (r poorReader) Read(p []byte) (int, error) {
	return 0, errors.New("failure to read")
}

// Return a pair of connections made of pipes, in which the first
// connection will fail Reads.
func faultyPipes() (io.ReadWriteCloser, io.ReadWriteCloser) {
	type end struct {
		io.Reader
		io.WriteCloser
	}

	serverReader, clientWriter := io.Pipe()
	_, serverWriter := io.Pipe()
	return end{poorReader{}, clientWriter}, end{serverReader, serverWriter}
}

func TestBadRPC(t *testing.T) {
	ctx := context.Background()
	mock := &remote.MockServer{}
	clientConn, serverConn := faultyPipes()
	server, err := NewServer(mock, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	go server.ServeConn(serverConn)

	client := NewClientV9(clientConn)
	if err = client.Ping(ctx); err == nil {
		t.Error("expected error from RPC system, got nil")
	}
	if _, ok := err.(remote.FatalError); !ok {
		t.Errorf("expected remote.FatalError from RPC mechanism, got %s", reflect.TypeOf(err))
	}
}
