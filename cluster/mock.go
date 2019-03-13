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
	AllWorkloadsFunc   func(maybeNamespace string) ([]Workload, error)
	SomeWorkloadsFunc  func([]flux.ResourceID) ([]Workload, error)
	PingFunc           func() error
	ExportFunc         func() ([]byte, error)
	SyncFunc           func(SyncSet) error
	PublicSSHKeyFunc   func(regenerate bool) (ssh.PublicKey, error)
	UpdateImageFunc    func(def []byte, id flux.ResourceID, container string, newImageID image.Ref) ([]byte, error)
	LoadManifestsFunc  func(base string, paths []string) (map[string]resource.Resource, error)
	UpdatePoliciesFunc func([]byte, flux.ResourceID, policy.Update) ([]byte, error)
}

func (m *Mock) AllWorkloads(maybeNamespace string) ([]Workload, error) {
	return m.AllWorkloadsFunc(maybeNamespace)
}

func (m *Mock) SomeWorkloads(s []flux.ResourceID) ([]Workload, error) {
	return m.SomeWorkloadsFunc(s)
}

func (m *Mock) Ping() error {
	return m.PingFunc()
}

func (m *Mock) Export() ([]byte, error) {
	return m.ExportFunc()
}

func (m *Mock) Sync(c SyncSet) error {
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

func (m *Mock) UpdatePolicies(def []byte, id flux.ResourceID, p policy.Update) ([]byte, error) {
	return m.UpdatePoliciesFunc(def, id, p)
}
