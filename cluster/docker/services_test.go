package docker

import (
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"testing"
)

func TestAllServices(t *testing.T) {
	c, err := NewSwarm(log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	c.AllServices("default_swarm", nil)
}

func TestSomeServices(t *testing.T) {
	c, err := NewSwarm(log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	var ids []flux.ServiceID
	c.SomeServices(ids)
}
