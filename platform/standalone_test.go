package platform

import (
	"errors"
	"testing"
	"time"

	"github.com/weaveworks/flux"
)

func TestStandaloneMessageBus(t *testing.T) {
	instID := flux.InstanceID("instance")
	bus := NewStandaloneMessageBus()
	p := &MockPlatform{}

	done := make(chan error)
	bus.Subscribe(instID, p, done)

	if err := bus.Ping(instID); err != nil {
		t.Fatal(err)
	}

	// subscribing another connection kicks the first one off
	p2 := &MockPlatform{PingError: errors.New("ping failed")}
	done2 := make(chan error)
	bus.Subscribe(instID, p2, done2)

	select {
	case <-done:
		break
	case <-time.After(1 * time.Second):
		t.Error("expected connection to be kicked when subsequent connection arrived, but it wasn't")
	}

	err := bus.Ping(instID)
	if err == nil {
		t.Error("expected error from pinging mock platform, but got nil")
	}
	select {
	case <-done2:
		break
	case <-time.After(1 * time.Second):
		t.Error("expected error from connection connection on error, got none")
	}
}
