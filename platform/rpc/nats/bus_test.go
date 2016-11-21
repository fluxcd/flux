package nats

import (
	"errors"

	"flag"
	"testing"
	"time"

	"github.com/nats-io/nats"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
)

var testNATS = flag.String("nats-url", "", "NATS connection URL; use NATS' default if empty")

func setup(t *testing.T) *NATS {
	flag.Parse()
	if *testNATS == "" {
		*testNATS = nats.DefaultURL
	}

	bus, err := NewMessageBus(*testNATS)
	if err != nil {
		t.Fatal(err)
	}
	return bus
}

func subscribe(t *testing.T, bus *NATS, errc chan error, inst flux.InstanceID, plat platform.Platform) {
	bus.Subscribe(inst, plat, errc)
	if err := bus.AwaitPresence(inst, 5*time.Second); err != nil {
		t.Fatal("Timed out waiting for instance to subscribe")
	}
}

func TestPing(t *testing.T) {
	bus := setup(t)

	errc := make(chan error)
	instID := flux.InstanceID("wirey-bird-68")
	platA := &platform.MockPlatform{}
	subscribe(t, bus, errc, instID, platA)

	// AwaitPresence uses Ping, so we have to install our error after
	// subscribe succeeds.
	platA.PingError = errors.New("ping problem")
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

	errc := make(chan error)

	instA := flux.InstanceID("steamy-windows-89")
	mockA := &platform.MockPlatform{
		AllServicesAnswer: []platform.Service{platform.Service{}},
		RegradeError:      platform.RegradeError{flux.ServiceID("foo/bar"): errors.New("foo barred")},
	}
	subscribe(t, bus, errc, instA, mockA)

	plat, err := bus.Connect(instA)
	if err != nil {
		t.Fatal(err)
	}
	ss, err := plat.AllServices("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(mockA.AllServicesAnswer) != len(ss) {
		t.Fatalf("Expected %d result, got %d", len(mockA.AllServicesAnswer), len(ss))
	}

	err = plat.Regrade([]platform.RegradeSpec{})
	if _, ok := err.(platform.RegradeError); !ok {
		t.Fatalf("expected RegradeError, got %+v", err)
	}

	mockB := &platform.MockPlatform{
		AllServicesError:   errors.New("just didn't feel like it"),
		SomeServicesAnswer: []platform.Service{platform.Service{}, platform.Service{}},
	}
	instB := flux.InstanceID("smokey-water-72")
	subscribe(t, bus, errc, instB, mockB)
	platB, err := bus.Connect(instB)
	if err != nil {
		t.Fatal(err)
	}

	ss, err = platB.SomeServices([]flux.ServiceID{})
	if err != nil {
		t.Fatal(err)
	}
	if len(mockB.SomeServicesAnswer) != len(ss) {
		t.Fatalf("Expected %d result, got %d", len(mockB.SomeServicesAnswer), len(ss))
	}

	ss, err = platB.AllServices("", nil)
	if err == nil {
		t.Fatal("expected error but didn't get one")
	}

	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("expected error return from subscription but didn't get one")
		}
	default:
		t.Fatal("expected error return from subscription but didn't get one")
	}
}
