package kubernetes

import (
	"sort"
	"testing"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
)

type mockApplier struct {
	commandRun bool
}

func (m *mockApplier) apply(_ log.Logger, c changeSet, errored map[flux.ResourceID]error) cluster.SyncError {
	if len(c.objs) != 0 {
		m.commandRun = true
	}
	return nil
}

type rsc struct {
	id    string
	bytes []byte
}

func (r rsc) ResourceID() flux.ResourceID {
	return flux.MustParseResourceID(r.id)
}

func (r rsc) Bytes() []byte {
	return r.bytes
}

func (r rsc) Policy() policy.Set {
	return nil
}

func (r rsc) Source() string {
	return "test"
}

// ---

func setup(t *testing.T) (*Cluster, *mockApplier) {
	applier := &mockApplier{}
	kube := &Cluster{
		applier: applier,
		logger:  log.NewNopLogger(),
	}
	return kube, applier
}

func TestSyncNop(t *testing.T) {
	kube, mock := setup(t)
	if err := kube.Sync(cluster.SyncDef{}); err != nil {
		t.Errorf("%#v", err)
	}
	if mock.commandRun {
		t.Error("expected no commands run")
	}
}

func TestSyncMalformed(t *testing.T) {
	kube, mock := setup(t)
	err := kube.Sync(cluster.SyncDef{
		Actions: []cluster.SyncAction{
			cluster.SyncAction{
				Apply: rsc{"default:deployment/trash", []byte("garbage")},
			},
		},
	})
	if err == nil {
		t.Error("expected error because malformed resource def, but got nil")
	}
	if mock.commandRun {
		t.Error("expected no commands run")
	}
}

// TestApplyOrder checks that applyOrder works as expected.
func TestApplyOrder(t *testing.T) {
	objs := []*apiObject{
		{
			Kind: "Deployment",
			Metadata: metadata{
				Name: "deploy",
			},
		},
		{
			Kind: "Secret",
			Metadata: metadata{
				Name: "secret",
			},
		},
		{
			Kind: "Namespace",
			Metadata: metadata{
				Name: "namespace",
			},
		},
	}
	sort.Sort(applyOrder(objs))
	for i, name := range []string{"namespace", "secret", "deploy"} {
		if objs[i].Metadata.Name != name {
			t.Errorf("Expected %q at position %d, got %q", name, i, objs[i].Metadata.Name)
		}
	}
}
