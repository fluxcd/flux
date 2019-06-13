package mock

import (
	"bytes"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/manifests"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/ssh"
)

// Doubles as a cluster.Cluster and cluster.Manifests implementation
type Mock struct {
	AllWorkloadsFunc              func(maybeNamespace string) ([]cluster.Workload, error)
	SomeWorkloadsFunc             func([]flux.ResourceID) ([]cluster.Workload, error)
	IsAllowedResourceFunc         func(flux.ResourceID) bool
	PingFunc                      func() error
	ExportFunc                    func() ([]byte, error)
	SyncFunc                      func(cluster.SyncSet) error
	PublicSSHKeyFunc              func(regenerate bool) (ssh.PublicKey, error)
	SetWorkloadContainerImageFunc func(def []byte, id flux.ResourceID, container string, newImageID image.Ref) ([]byte, error)
	LoadManifestsFunc             func(base string, paths []string) (map[string]resource.Resource, error)
	ParseManifestFunc             func(def []byte, source string) (map[string]resource.Resource, error)
	UpdateWorkloadPoliciesFunc    func([]byte, flux.ResourceID, policy.Update) ([]byte, error)
	CreateManifestPatchFunc       func(originalManifests, modifiedManifests []byte, originalSource, modifiedSource string) ([]byte, error)
	ApplyManifestPatchFunc        func(originalManifests, patch []byte, originalSource, patchSource string) ([]byte, error)
	AppendManifestToBufferFunc    func([]byte, *bytes.Buffer) error
}

var _ cluster.Cluster = &Mock{}
var _ manifests.Manifests = &Mock{}

func (m *Mock) AllWorkloads(maybeNamespace string) ([]cluster.Workload, error) {
	return m.AllWorkloadsFunc(maybeNamespace)
}

func (m *Mock) SomeWorkloads(s []flux.ResourceID) ([]cluster.Workload, error) {
	return m.SomeWorkloadsFunc(s)
}

func (m *Mock) IsAllowedResource(id flux.ResourceID) bool {
	return m.IsAllowedResourceFunc(id)
}

func (m *Mock) Ping() error {
	return m.PingFunc()
}

func (m *Mock) Export() ([]byte, error) {
	return m.ExportFunc()
}

func (m *Mock) Sync(c cluster.SyncSet) error {
	return m.SyncFunc(c)
}

func (m *Mock) PublicSSHKey(regenerate bool) (ssh.PublicKey, error) {
	return m.PublicSSHKeyFunc(regenerate)
}

func (m *Mock) SetWorkloadContainerImage(def []byte, id flux.ResourceID, container string, newImageID image.Ref) ([]byte, error) {
	return m.SetWorkloadContainerImageFunc(def, id, container, newImageID)
}

func (m *Mock) LoadManifests(baseDir string, paths []string) (map[string]resource.Resource, error) {
	return m.LoadManifestsFunc(baseDir, paths)
}

func (m *Mock) ParseManifest(def []byte, source string) (map[string]resource.Resource, error) {
	return m.ParseManifestFunc(def, source)
}

func (m *Mock) UpdateWorkloadPolicies(def []byte, id flux.ResourceID, p policy.Update) ([]byte, error) {
	return m.UpdateWorkloadPoliciesFunc(def, id, p)
}

func (m *Mock) CreateManifestPatch(originalManifests, modifiedManifests []byte, originalSource, modifiedSource string) ([]byte, error) {
	return m.CreateManifestPatchFunc(originalManifests, modifiedManifests, originalSource, modifiedSource)
}

func (m *Mock) ApplyManifestPatch(originalManifests, patch []byte, originalSource, patchSource string) ([]byte, error) {
	return m.ApplyManifestPatch(originalManifests, patch, originalSource, patchSource)
}

func (m *Mock) AppendManifestToBuffer(b []byte, buf *bytes.Buffer) error {
	return m.AppendManifestToBuffer(b, buf)
}
