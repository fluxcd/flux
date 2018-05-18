package cluster

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/ssh"
)

// Doubles as a cluster.Cluster and cluster.Manifests implementation
type Mock struct {
	AllServicesFunc          func(maybeNamespace string) ([]Controller, error)
	SomeServicesFunc         func([]flux.ResourceID) ([]Controller, error)
	PingFunc                 func() error
	ExportFunc               func() ([]byte, error)
	SyncFunc                 func(SyncDef) error
	PublicSSHKeyFunc         func(regenerate bool) (ssh.PublicKey, error)
	FindDefinedServicesFunc  func(path string) (map[flux.ResourceID][]string, error)
	UpdateImageFunc          func(path string, id flux.ResourceID, container string, newImageID image.Ref) error
	LoadManifestsFunc        func(base, first string, rest ...string) (map[string]resource.Resource, error)
	ParseManifestsFunc       func([]byte) (map[string]resource.Resource, error)
	UpdatePoliciesFunc       func(path string, id flux.ResourceID, update policy.Update) error
	ServicesWithPoliciesFunc func(path string) (policy.ResourceMap, error)
}

func (m *Mock) AllControllers(maybeNamespace string) ([]Controller, error) {
	return m.AllServicesFunc(maybeNamespace)
}

func (m *Mock) SomeControllers(s []flux.ResourceID) ([]Controller, error) {
	return m.SomeServicesFunc(s)
}

func (m *Mock) Ping() error {
	return m.PingFunc()
}

func (m *Mock) Export() ([]byte, error) {
	return m.ExportFunc()
}

func (m *Mock) Sync(c SyncDef) error {
	return m.SyncFunc(c)
}

func (m *Mock) PublicSSHKey(regenerate bool) (ssh.PublicKey, error) {
	return m.PublicSSHKeyFunc(regenerate)
}

func (m *Mock) FindDefinedServices(path string) (map[flux.ResourceID][]string, error) {
	return m.FindDefinedServicesFunc(path)
}

func (m *Mock) UpdateImage(path string, id flux.ResourceID, container string, newImageID image.Ref) error {
	return m.UpdateImageFunc(path, id, container, newImageID)
}

func (m *Mock) LoadManifests(base, first string, rest ...string) (map[string]resource.Resource, error) {
	return m.LoadManifestsFunc(base, first, rest...)
}

func (m *Mock) ParseManifests(def []byte) (map[string]resource.Resource, error) {
	return m.ParseManifestsFunc(def)
}

func (m *Mock) UpdatePolicies(path string, id flux.ResourceID, p policy.Update) error {
	return m.UpdatePoliciesFunc(path, id, p)
}

func (m *Mock) ServicesWithPolicies(path string) (policy.ResourceMap, error) {
	return m.ServicesWithPoliciesFunc(path)
}
