package bus

import (
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/service"
)

// For getting a connection to a platform; this can happen in
// different ways, e.g., by having direct access to Kubernetes in
// standalone mode, or by going via a message bus.
type Connecter interface {
	// Connect returns a platform for the instance specified. An error
	// is returned only if there is a problem (possibly transient)
	// with the underlying mechanism (i.e., not if the platform is
	// simply not known to be connected at this time).
	Connect(inst service.InstanceID) (remote.Platform, error)
}

// MessageBus handles routing messages to/from the matching platform.
type MessageBus interface {
	Connecter
	// Subscribe registers a platform as the daemon for the instance
	// specified.
	Subscribe(inst service.InstanceID, p remote.Platform, done chan<- error)
	// Ping returns nil if the daemon for the instance given is known
	// to be connected, or ErrPlatformNotAvailable otherwise. NB this
	// differs from the semantics of `Connecter.Connect`.
	Ping(inst service.InstanceID) error
}
