// Package registry provides domain abstractions over container registries.
package registry

import "time"

// Registry represents a container registry from the perspective of a specific,
// logged-in user or organization.
type Registry interface {
	Repositories() []Repository
}

// Repository is a collection of related (i.e. identical) images.
type Repository interface {
	Name() string // "weaveworks/helloworld"
	Images() []Image
}

// Image represents a specific container image available in a repository. It's a
// struct because I think we can safely assume the data here is pretty
// universal across different registries and repositories.
type Image struct {
	Name      string    // "weaveworks/helloworld"
	Tag       string    // "master-59f0001"
	CreatedAt time.Time // Always UTC
}
