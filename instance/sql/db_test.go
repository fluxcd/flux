package sql

import (
	"errors"
	"testing"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/instance"
)

func TestNew(t *testing.T) {
	_, err := New("ql-mem", "config")
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateOK(t *testing.T) {
	inst := flux.InstanceID("floaty-womble-abc123")
	service := flux.MakeServiceID("namespace", "service")

	db, err := New("ql-mem", "config")
	if err != nil {
		t.Fatal(err)
	}

	services := map[flux.ServiceID]instance.ServiceConfig{}
	services[service] = instance.ServiceConfig{
		Automated: true,
	}
	c := instance.Config{
		Services: services,
	}
	err = db.Update(inst, func(_ instance.Config) (instance.Config, error) {
		return c, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	c1, err := db.Get(inst)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := c1.Services[service]; !found {
		t.Fatalf("did not find instance config after setting")
	}
	if !c1.Services[service].Automated {
		t.Fatalf("expected service config %#v, got %#v", c.Services[service], c1.Services[service])
	}
}

func TestUpdateRollback(t *testing.T) {
	inst := flux.InstanceID("floaty-womble-abc123")
	service := flux.MakeServiceID("namespace", "service")

	db, err := New("ql-mem", "config")
	if err != nil {
		t.Fatal(err)
	}

	services := map[flux.ServiceID]instance.ServiceConfig{}
	services[service] = instance.ServiceConfig{
		Automated: true,
	}
	c := instance.Config{
		Services: services,
	}

	err = db.Update(inst, func(_ instance.Config) (instance.Config, error) {
		return instance.Config{}, errors.New("arbitrary fail")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	c1, err := db.Get(inst)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := c1.Services[service]; !found {
		t.Fatalf("did not find instance config after setting")
	}
	if !c1.Services[service].Automated {
		t.Fatalf("expected service config %#v, got %#v", c.Services[service], c1.Services[service])
	}
}
