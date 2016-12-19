package rpc

import (
	"errors"
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
	serviceList := []flux.ServiceID{serviceID}
	services := flux.ServiceIDSet{}
	services.Add(serviceList)

	releases := []platform.ReleaseSpec{
		platform.ReleaseSpec{
			ServiceID:     serviceID,
			NewDefinition: []byte("imagine a definition here"),
		},
	}

	serviceAnswer := []platform.Service{
		platform.Service{
			ID:       flux.ServiceID("foobar/hello"),
			IP:       "10.32.1.45",
			Metadata: map[string]string{},
			Status:   "ok",
			Containers: platform.ContainersOrExcuse{
				Containers: []platform.Container{
					platform.Container{
						Name:  "frobnicator",
						Image: "quay.io/example.com/frob:v0.4.5",
					},
				},
			},
		},
		platform.Service{},
	}

	mock := &platform.MockPlatform{
		AllServicesArgTest: func(ns string, ss flux.ServiceIDSet) error {
			if !(ns == namespace &&
				ss.Contains(serviceID)) {
				return fmt.Errorf("did not get expected args, got %q, %+v", ns, ss)
			}
			return nil
		},
		AllServicesAnswer: serviceAnswer,

		SomeServicesArgTest: func(ss []flux.ServiceID) error {
			if !reflect.DeepEqual(ss, serviceList) {
				return fmt.Errorf("did not get expected args, got %+v", ss)
			}
			return nil
		},
		SomeServicesAnswer: serviceAnswer,

		ReleaseArgTest: func(specs []platform.ReleaseSpec) error {
			if !reflect.DeepEqual(releases, specs) {
				return fmt.Errorf("did not get expected args, got %+v", specs)
			}
			return nil
		},
		ReleaseError: nil,
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
	mock.AllServicesError = fmt.Errorf("all services query failure")
	ss, err = client.AllServices(namespace, services)
	if err == nil {
		t.Error("expected error, got nil")
	}

	ss, err = client.SomeServices(serviceList)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(ss, mock.SomeServicesAnswer) {
		t.Error(fmt.Errorf("expected %d result(s), got %+v", len(mock.SomeServicesAnswer), ss))
	}
	mock.SomeServicesError = fmt.Errorf("fail for some reason")
	ss, err = client.SomeServices(serviceList)
	if err == nil {
		t.Error("expected error, got nil")
	}

	err = client.Release(releases)
	if err != nil {
		t.Error(err)
	}

	releaseErrors := platform.ReleaseError{
		serviceID: fmt.Errorf("it just failed"),
	}
	mock.ReleaseError = releaseErrors
	err = client.Release(releases)
	if !reflect.DeepEqual(err, releaseErrors) {
		t.Errorf("expected ReleaseError, got %#v", err)
	}
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
	mock := &platform.MockPlatform{}
	clientConn, serverConn := faultyPipes()
	server, err := NewServer(mock)
	if err != nil {
		t.Fatal(err)
	}
	go server.ServeConn(serverConn)

	client := NewClient(clientConn)
	if err = client.Ping(); err == nil {
		t.Error("expected error from RPC system, got nil")
	}
	if _, ok := err.(platform.FatalError); !ok {
		t.Errorf("expected platform.FatalError from RPC mechanism, got %s", reflect.TypeOf(err))
	}
}
