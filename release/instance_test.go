package release

import (
	"testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
)

func TestLockedServices(t *testing.T) {
	conf := instance.Config{
		Services: map[flux.ServiceID]instance.ServiceConfig{
			flux.ServiceID("service1"): instance.ServiceConfig{
				Locked: true,
			},
			flux.ServiceID("service2"): instance.ServiceConfig{
				Locked:    true,
				Automated: true,
			},
			flux.ServiceID("service3"): instance.ServiceConfig{
				Automated: true,
			},
		},
	}

	locked := LockedServices(conf)
	if !locked.Contains(flux.ServiceID("service1")) {
		t.Error("service1 locked in config but not reported as locked")
	}
	if !locked.Contains(flux.ServiceID("service2")) {
		t.Error("service2 locked in config but not reported as locked")
	}
	if locked.Contains(flux.ServiceID("service3")) {
		t.Error("service3 not locked but reported as locked")
	}
}
