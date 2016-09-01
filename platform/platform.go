// Package platform will hold abstractions and data types common to supported
// platforms. We don't know what all of those will look like, yet. So the
// package is mostly empty.
package platform

import (
	"github.com/pkg/errors"
)

// Service describes a platform service, generally a floating IP with one or
// more exposed ports that map to a load-balanced pool of instances. Eventually
// this type will generalize to something of a lowest-common-denominator for
// all supported platforms, but right now it looks a lot like a Kubernetes
// service.
type Service struct {
	Name     string
	IP       string
	Metadata map[string]string // a grab bag of goodies, likely platform-specific
	Status   string            // A status summary for display
}

// A Container represents a container specification in a pod. The Name
// identifies it within the pod, and the Image says which image it's
// configured to run.
type Container struct {
	Name  string
	Image string
}

// These errors all represent logical problems with platform
// configuration, and may be recoverable; e.g., it might be fine if a
// service does not have a matching RC/deployment.
type Error struct{ error }

var (
	ErrEmptySelector        = &Error{errors.New("empty selector")}
	ErrWrongResourceKind    = &Error{errors.New("new definition does not match existing resource")}
	ErrNoMatchingService    = &Error{errors.New("no matching service")}
	ErrServiceHasNoSelector = &Error{errors.New("service has no selector")}
	ErrNoMatching           = &Error{errors.New("no matching replication controllers or deployments")}
	ErrMultipleMatching     = &Error{errors.New("multiple matching replication controllers or deployments")}
	ErrNoMatchingImages     = &Error{errors.New("no matching images")}
)

// Construct a platform error that wraps an error from the platform
// itself; these are not supposed to be recoverable.  Special case: if
// given `nil`, will return nil, for convenience when returning
// error|nil.
func WrapError(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &Error{errors.Wrap(err, msg)}
}
