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
	AllServicesFunc    func(maybeNamespace string) ([]Controller, error)
	SomeServicesFunc   func([]flux.ResourceID) ([]Controller, error)
	PingFunc           func() error
	ExportFunc         func() ([]byte, error)
	SyncFunc           func(SyncDef) error
	PublicSSHKeyFunc   func(regenerate bool) (ssh.PublicKey, error)
	UpdateImageFunc    func(def []byte, id flux.ResourceID, container string, newImageID image.Ref) ([]byte, error)
	LoadManifestsFunc  func(base string, paths []string) (map[string]resource.Resource, error)
	ParseManifestsFunc func([]byte) (map[string]resource.Resource, error)
	UpdateManifestFunc func(path, resourceID string, f func(def []byte) ([]byte, error)) error
	UpdatePoliciesFunc func([]byte, flux.ResourceID, policy.Update) ([]byte, error)
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

func (m *Mock) Sync(c SyncDef, errored map[flux.ResourceID]error) error {
	return m.SyncFunc(c)
}

func (m *Mock) PublicSSHKey(regenerate bool) (ssh.PublicKey, error) {
	return m.PublicSSHKeyFunc(regenerate)
}

func (m *Mock) UpdateImage(def []byte, id flux.ResourceID, container string, newImageID image.Ref) ([]byte, error) {
	return m.UpdateImageFunc(def, id, container, newImageID)
}

func (m *Mock) LoadManifests(base string, paths []string) (map[string]resource.Resource, error) {
	return m.LoadManifestsFunc(base, paths)
}

func (m *Mock) ParseManifests(def []byte) (map[string]resource.Resource, error) {
	return m.ParseManifestsFunc(def)
}

func (m *Mock) UpdateManifest(path string, resourceID string, f func(def []byte) ([]byte, error)) error {
	return m.UpdateManifestFunc(path, resourceID, f)
}

func (m *Mock) UpdatePolicies(def []byte, id flux.ResourceID, p policy.Update) ([]byte, error) {
	return m.UpdatePoliciesFunc(def, id, p)
}
