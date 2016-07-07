// Package swarm provides Swarm-flavored platform objects.
package swarm

import "github.com/weaveworks/fluxy/platform"

// Platform represents a single Swarm cluster, as understood by a fluxd daemon
// deployed within it. That means something like a connection to the API server,
// so state may be queried.
type Platform struct{}

// Services implements platform.Platform.
func (p *Platform) Services() []platform.Service {
	return []platform.Service{}
}

// Service represents a single Swarm service as yielded by a platform.
type Service struct {
	name      string
	instances []Instance
}

// Name implements platform.Service.
func (s *Service) Name() string {
	return s.name
}

// Instances implements platform.Service.
func (s *Service) Instances() []platform.Instance {
	res := make([]platform.Instance, len(s.instances))
	for i := range s.instances {
		res[i] = &s.instances[i]
	}
	return res
}

// Instance represents a single container, or task, in a Swarm service.
type Instance struct {
	id string
}

// ID implements platform.Instance.
func (i *Instance) ID() string {
	return i.id
}
