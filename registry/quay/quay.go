// Package quay provides Quay.io-flavored registry objects.
package quay

import (
	"github.com/weaveworks/fluxy/registry"
)

// Registry represents a single Quay registry.
type Registry struct {
	// Some Quay auth details stored here
}

// NewRegistry returns a new Quay-flavored Registry.
func NewRegistry(username, password string) *Registry {
	return &Registry{}
}

// Repositories implements registry.Registry.
func (r *Registry) Repositories() []registry.Repository {
	return []registry.Repository{}
}

// Repository represents a single Quay repository, yielded by a Quay Registry.
type Repository struct {
	name   string
	images []registry.Image
}

// Name implements registry.Repository.
func (r *Repository) Name() string {
	return r.name
}

// Images implements registry.Repository.
func (r *Repository) Images() []registry.Image {
	return r.images
}
