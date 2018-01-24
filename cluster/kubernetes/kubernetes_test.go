package kubernetes

import (
	"testing"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/cluster"
)

type mockApplier struct {
	commandRun bool
}

func (m *mockApplier) apply(_ log.Logger, c changeSet, _ cluster.SyncError) {
	if len(c.nsObjs) != 0 || len(c.noNsObjs) != 0 {
		m.commandRun = true
	}
}

func deploymentDef(name string) []byte {
	return []byte(`---
kind: Deployment
metadata:
  name: ` + name)
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
		t.Error(err)
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
				ResourceID: "foobar",
				Apply:      []byte("garbage"),
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
