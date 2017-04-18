package fluxd

import (
	"github.com/weaveworks/fluxy/flux"
)

// Service captures the behavior expected of a fluxd, which is spoken to only
// thru fluxsvc. For now, that's the minimal set of things necessary to run
// against a platform.
type Service interface {
	ServiceLister
	ImageLister
	Releaser
}

// ServiceLister lists services on a platform.
type ServiceLister interface {
	ListServices(namespace string) ([]flux.ServiceStatus, error)
}

// ImageLister lists images on a platform.
type ImageLister interface {
	ListImages(flux.ServiceSpec) ([]flux.ImageStatus, error)
}

// Releaser executes releases against a platform.
type Releaser interface {
	Release(flux.ServiceSpec, flux.ImageSpec, flux.ReleaseKind) ([]flux.ReleaseAction, error)
}
