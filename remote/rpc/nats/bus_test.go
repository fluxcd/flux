package nats

import (
	"errors"

	"flag"
	"testing"
	"time"

	"github.com/nats-io/nats"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/remote"
)

var testNATS = flag.String("nats-url", "", "NATS connection URL; use NATS' default if empty")

var metrics = remote.BusMetricsImpl

func setup(t *testing.T) *NATS {
	flag.Parse()
	if *testNATS == "" {
		*testNATS = nats.DefaultURL
	}

	bus, err := NewMessageBus(*testNATS, metrics)
	if err != nil {
		t.Fatal(err)
	}
	return bus
}

func subscribe(t *testing.T, bus *NATS, errc chan error, inst flux.InstanceID, plat remote.Platform) {
	bus.Subscribe(inst, plat, errc)
	if err := bus.AwaitPresence(inst, 5*time.Second); err != nil {
		t.Fatal("Timed out waiting for instance to subscribe")
	}
}

func TestPing(t *testing.T) {
	bus := setup(t)
	errc := make(chan error)
	instID := flux.InstanceID("wirey-bird-68")
	platA := &remote.MockPlatform{}
	subscribe(t, bus, errc, instID, platA)

	// AwaitPresence uses Ping, so we have to install our error after
	// subscribe succeeds.
	platA.PingError = remote.FatalError{errors.New("ping problem")}
	if err := platA.Ping(); err == nil {
		t.Fatalf("expected error from directly calling ping, got nil")
	}

	err := bus.Ping(instID)
	if err == nil {
		t.Errorf("expected error from ping, got nil")
	} else if err.Error() != "ping problem" {
		t.Errorf("got the wrong error: %s", err.Error())
	}

	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("expected error return from subscription but didn't get one")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected error return from subscription but didn't get one")
	}
}

func TestMethods(t *testing.T) {
	bus := setup(t)
	errc := make(chan error, 1)
	instA := flux.InstanceID("steamy-windows-89")

	wrap := func(mock remote.Platform) remote.Platform {
		subscribe(t, bus, errc, instA, mock)
		plat, err := bus.Connect(instA)
		if err != nil {
			t.Fatal(err)
		}
		return plat
	}
	remote.PlatformTestBattery(t, wrap)

	close(errc)
	err := <-errc
	if err != nil {
		t.Fatalf("expected nil from subscription channel, but got err %v", err)
	}
}

func TestFatalErrorDisconnects(t *testing.T) {
	bus := setup(t)

	errc := make(chan error)

	instA := flux.InstanceID("golden-years-75")
	mockA := &remote.MockPlatform{
		ListServicesError: remote.FatalError{errors.New("Disaster.")},
	}
	subscribe(t, bus, errc, instA, mockA)

	plat, err := bus.Connect(instA)
	if err != nil {
		t.Fatal(err)
	}

	_, err = plat.ListServices("")
	if err == nil {
		t.Error("expected error, got nil")
	} else if _, ok := err.(remote.FatalError); !ok {
		t.Errorf("expected remote.FatalError, got %v", err)
	}

	select {
	case err = <-errc:
		if err == nil {
			t.Error("expected error from subscription being killed, got nil")
		}
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for expected error from subscription closing")
	}
}

func TestNewConnectionKicks(t *testing.T) {
	bus := setup(t)

	instA := flux.InstanceID("breaky-chain-77")

	mockA := &remote.MockPlatform{}
	errA := make(chan error)
	subscribe(t, bus, errA, instA, mockA)

	mockB := &remote.MockPlatform{}
	errB := make(chan error)
	subscribe(t, bus, errB, instA, mockB)

	select {
	case <-errA:
		break
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for connection to be kicked")
	}

	close(errB)
	err := <-errB
	if err != nil {
		t.Errorf("expected no error from second connection, but got %q", err)
	}
}
