// Package platform provides domain abstractions over orchestration platforms.
package platform

// Platform describes a runtime platform, like Swarm.
type Platform interface {
	Services() []Service
}

// Service describes a single service as understood by a platform. This is an
// interface rather than a struct, because we want to allow platforms to define
// their own Service structs with all of their specific implementation details.
type Service interface {
	Name() string
	Instances() []Instance
}

// Instance describes a single physical instance as understood by a platform.
// The same caveats re: interface apply as to the service.
type Instance interface {
	ID() string
}
