package platform

import (
	"errors"
	"testing"

	"github.com/weaveworks/flux"
)

// Just tests that the mock does its job.
func TestPlatformMock(t *testing.T) {
	var p = &MockPlatform{
		AllServicesAnswer: []Service{Service{}},
		SomeServicesArgTest: func([]flux.ServiceID) error {
			return errors.New("arg fail")
		},
		ApplyError: errors.New("fail apply"),
		SyncError:  errors.New("fail sync"),
	}
	var _ Platform = p

	// Just token tests so we're attempting _something_ here
	ss, err := p.AllServices("", flux.ServiceIDSet{})
	if err != nil {
		t.Error(err)
	}
	if len(ss) != 1 {
		t.Errorf("expected answer given in mock, but got %+v", ss)
	}

	ss, err = p.SomeServices([]flux.ServiceID{})
	if err == nil {
		t.Error("expected error from args test, got nil")
	}

	err = p.Apply([]ServiceDefinition{})
	if err == nil {
		t.Error("expected error, got nil")
	}

	p.ApplyError = nil
	p.ApplyArgTest = func([]ServiceDefinition) error {
		return errors.New("apply args fail")
	}
	if err = p.Apply([]ServiceDefinition{}); err == nil {
		t.Error("expected error from apply, got nil")
	}

	if err = p.Sync(SyncDef{}); err == nil {
		t.Error("expected error from sync, got nil")
	}

	p.SyncError = nil
	p.SyncArgTest = func(SyncDef) error {
		return errors.New("sync args fail")
	}
	if err = p.Sync(SyncDef{}); err == nil {
		t.Error("expected error from sync, got nil")
	}
}
