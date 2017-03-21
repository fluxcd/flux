package platform

import (
	"github.com/weaveworks/flux"
)

type MockPlatform struct {
	AllServicesArgTest func(string, flux.ServiceIDSet) error
	AllServicesAnswer  []Service
	AllServicesError   error

	SomeServicesArgTest func([]flux.ServiceID) error
	SomeServicesAnswer  []Service
	SomeServicesError   error

	ApplyArgTest func([]ServiceDefinition) error
	ApplyError   error

	PingError error

	VersionAnswer string
	VersionError  error

	ExportAnswer []byte
	ExportError  error
}

func (p *MockPlatform) AllServices(ns string, ss flux.ServiceIDSet) ([]Service, error) {
	if p.AllServicesArgTest != nil {
		if err := p.AllServicesArgTest(ns, ss); err != nil {
			return nil, err
		}
	}
	return p.AllServicesAnswer, p.AllServicesError
}

func (p *MockPlatform) SomeServices(ss []flux.ServiceID) ([]Service, error) {
	if p.SomeServicesArgTest != nil {
		if err := p.SomeServicesArgTest(ss); err != nil {
			return nil, err
		}
	}
	return p.SomeServicesAnswer, p.SomeServicesError
}

func (p *MockPlatform) Apply(defs []ServiceDefinition) error {
	if p.ApplyArgTest != nil {
		if err := p.ApplyArgTest(defs); err != nil {
			return err
		}
	}
	return p.ApplyError
}

func (p *MockPlatform) Ping() error {
	return p.PingError
}

func (p *MockPlatform) Version() (string, error) {
	return p.VersionAnswer, p.VersionError
}

func (p *MockPlatform) Export() ([]byte, error) {
	return p.ExportAnswer, p.ExportError
}
