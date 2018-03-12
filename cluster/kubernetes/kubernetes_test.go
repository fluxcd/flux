package kubernetes

import (
	"testing"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
)

type mockApplier struct {
	commandRun bool
}

func (m *mockApplier) apply(_ log.Logger, c changeSet) cluster.SyncError {
	if len(c.nsObjs) != 0 || len(c.noNsObjs) != 0 {
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
				Apply: rsc{"id", []byte("garbage")},
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
