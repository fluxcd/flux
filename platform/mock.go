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

	RegradeArgTest func([]RegradeSpec) error
	RegradeError   error

	PingError error
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

func (p *MockPlatform) Regrade(ss []RegradeSpec) error {
	if p.RegradeArgTest != nil {
		if err := p.RegradeArgTest(ss); err != nil {
			return err
		}
	}
	return p.RegradeError
}

func (p *MockPlatform) Ping() error {
	return p.PingError
}
