package fluxsvc

import (
	"github.com/weaveworks/fluxy/flux"
	"github.com/weaveworks/fluxy/flux/history"
)

// Service captures the behaviors expected of the fluxsvc, which is what fluxctl
// talks to. For now, that's a sort of superset of methods available on fluxd.
type Service interface {
	ServiceLister
	ImageLister
	Releaser
	Automator
	HistoryReader
}

// ServiceLister lists services on a platform identified by orgID.
type ServiceLister interface {
	ListServices(orgID string, namespace string) ([]flux.ServiceStatus, error)
}

// ImageLister lists images on a platform identified by orgID.
type ImageLister interface {
	ListImages(orgID string, s flux.ServiceSpec) ([]flux.ImageStatus, error)
}

// Releaser executes releases against a platform identified by orgID.
type Releaser interface {
	Release(orgID string, s flux.ServiceSpec, i flux.ImageSpec, k flux.ReleaseKind) ([]flux.ReleaseAction, error)
}

// Automator [de]automates services on a platform identified by orgID.
type Automator interface {
	Automate(orgID string, s flux.ServiceID) error
	Deautomate(orgID string, s flux.ServiceID) error
}

// HistoryReader reads history entries from a store parameterized by orgID.
type HistoryReader interface {
	History(orgID string, s flux.ServiceSpec, n int) ([]history.Event, error)
}
