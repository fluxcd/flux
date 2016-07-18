// Package platform will hold abstractions and data types common to supported
// platforms. We don't know what all of those will look like, yet. So the
// package is mostly empty.
package platform

// Service describes a platform service, generally a floating IP with one or
// more exposed ports that map to a load-balanced pool of instances. Eventually
// this type will generalize to something of a lowest-common-denominator for
// all supported platforms, but right now it looks a lot like a Kubernetes
// service.
type Service struct {
	Name     string
	Image    string // currently running, e.g. "quay.io/weaveworks/helloworld:master-a000001"
	IP       string
	Ports    []Port
	Metadata map[string]string // a grab bag of goodies, likely platform-specific
}

// Port describes the mapping of a port on a service IP to the corresponding
// port on load-balanced instances, including the protocol supported on that
// port. Ports are strings because Kubernetes defines its internal port
// (TargetPort) as something called an IntOrString.
type Port struct {
	External string // what is exposed to the world
	Internal string // what it maps to on the backends
	Protocol string // e.g. TCP, HTTP
}

// A Container represents a container specification in a pod. The Name
// identifies it within the pod, and the Image says which image it's
// configured to run.
type Container struct {
	Name  string
	Image string
}
