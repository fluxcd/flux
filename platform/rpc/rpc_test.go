package rpc

import (
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
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
	namespace := "space-of-names"
	serviceID := flux.ServiceID(namespace + "/service")
	services := flux.ServiceIDSet{}
	services.Add([]flux.ServiceID{serviceID})

	mock := &platform.MockPlatform{
		AllServicesArgTest: func(ns string, ss flux.ServiceIDSet) error {
			if !(ns == namespace &&
				ss.Contains(serviceID)) {
				return fmt.Errorf("did not get expected args, got %q, %+v", ns, ss)
			}
			return nil
		},
		AllServicesAnswer: []platform.Service{platform.Service{}, platform.Service{}},
	}

	clientConn, serverConn := pipes()

	server, err := NewServer(mock)
	if err != nil {
		t.Fatal(err)
	}
	go server.ServeConn(serverConn)

	client := NewClient(clientConn)
	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}

	ss, err := client.AllServices(namespace, services)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(ss, mock.AllServicesAnswer) {
		t.Error(fmt.Errorf("expected %d result(s), got %+v", len(mock.AllServicesAnswer), ss))
	}
}
