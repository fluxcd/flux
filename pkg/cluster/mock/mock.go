package mock

import (
	"bytes"
	"context"

	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/manifests"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/ssh"
)

// Doubles as a cluster.Cluster and cluster.Manifests implementation
type Mock struct {
	AllWorkloadsFunc              func(ctx context.Context, maybeNamespace string) ([]cluster.Workload, error)
	SomeWorkloadsFunc             func(ctx context.Context, ids []resource.ID) ([]cluster.Workload, error)
	IsAllowedResourceFunc         func(resource.ID) bool
	PingFunc                      func() error
	ExportFunc                    func(ctx context.Context) ([]byte, error)
	SyncFunc                      func(cluster.SyncSet) error
	PublicSSHKeyFunc              func(regenerate bool) (ssh.PublicKey, error)
	SetWorkloadContainerImageFunc func(def []byte, id resource.ID, container string, newImageID image.Ref) ([]byte, error)
	LoadManifestsFunc             func(base string, paths []string) (map[string]resource.Resource, error)
	ParseManifestFunc             func(def []byte, source string) (map[string]resource.Resource, error)
	UpdateWorkloadPoliciesFunc    func([]byte, resource.ID, resource.PolicyUpdate) ([]byte, error)
	CreateManifestPatchFunc       func(originalManifests, modifiedManifests []byte, originalSource, modifiedSource string) ([]byte, error)
	ApplyManifestPatchFunc        func(originalManifests, patch []byte, originalSource, patchSource string) ([]byte, error)
	AppendManifestToBufferFunc    func([]byte, *bytes.Buffer) error
}

var _ cluster.Cluster = &Mock{}
var _ manifests.Manifests = &Mock{}

func (m *Mock) AllWorkloads(ctx context.Context, maybeNamespace string) ([]cluster.Workload, error) {
	return m.AllWorkloadsFunc(ctx, maybeNamespace)
}

func (m *Mock) SomeWorkloads(ctx context.Context, ids []resource.ID) ([]cluster.Workload, error) {
	return m.SomeWorkloadsFunc(ctx, ids)
}

func (m *Mock) IsAllowedResource(id resource.ID) bool {
	return m.IsAllowedResourceFunc(id)
}

func (m *Mock) Ping() error {
	return m.PingFunc()
}

func (m *Mock) Export(ctx context.Context) ([]byte, error) {
	return m.ExportFunc(ctx)
}

func (m *Mock) Sync(c cluster.SyncSet) error {
	return m.SyncFunc(c)
}

func (m *Mock) PublicSSHKey(regenerate bool) (ssh.PublicKey, error) {
	return m.PublicSSHKeyFunc(regenerate)
}

func (m *Mock) SetWorkloadContainerImage(def []byte, id resource.ID, container string, newImageID image.Ref) ([]byte, error) {
	return m.SetWorkloadContainerImageFunc(def, id, container, newImageID)
}

func (m *Mock) LoadManifests(baseDir string, paths []string) (map[string]resource.Resource, error) {
	return m.LoadManifestsFunc(baseDir, paths)
}

func (m *Mock) ParseManifest(def []byte, source string) (map[string]resource.Resource, error) {
	return m.ParseManifestFunc(def, source)
}

func (m *Mock) UpdateWorkloadPolicies(def []byte, id resource.ID, p resource.PolicyUpdate) ([]byte, error) {
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
