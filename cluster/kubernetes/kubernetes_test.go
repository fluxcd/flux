package kubernetes

// Test that the implementation of platform wrt Kubernetes is
// adequate. Starting with Sync.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/go-kit/kit/log"
	"k8s.io/client-go/1.5/rest"

	"github.com/weaveworks/flux/cluster"
)

type command struct {
	action string
	def    string
}

type mockApplier struct {
	commands  []command
	applyErr  error
	createErr error
	deleteErr error
}

func (m *mockApplier) Apply(logger log.Logger, obj *apiObject) error {
	m.commands = append(m.commands, command{"apply", string(obj.Metadata.Name)})
	return m.applyErr
}

func (m *mockApplier) Delete(logger log.Logger, obj *apiObject) error {
	m.commands = append(m.commands, command{"delete", string(obj.Metadata.Name)})
	return m.deleteErr
}

func deploymentDef(name string) []byte {
	return []byte(`---
kind: Deployment
metadata:
  name: ` + name + `
  namespace: test-ns
`)
}

// ---

func setup(t *testing.T) (*Cluster, *mockApplier) {
	restClientConfig := &rest.Config{}
	applier := &mockApplier{}
	kube, err := NewCluster(restClientConfig, applier, log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	return kube, applier
}

func TestSyncNop(t *testing.T) {
	kube, mock := setup(t)
	if err := kube.Sync(cluster.SyncDef{}); err != nil {
		t.Error(err)
	}
	if len(mock.commands) > 0 {
		t.Errorf("expected no commands run, but got %#v", mock.commands)
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
	if len(mock.commands) > 0 {
		t.Errorf("expected no commands run, but got %#v", mock.commands)
	}
}

func TestSyncOrder(t *testing.T) {
	kube, mock := setup(t)
	if err := kube.Sync(cluster.SyncDef{
		Actions: []cluster.SyncAction{
			cluster.SyncAction{
				ResourceID: "foobar",
				Delete:     deploymentDef("delete first"),
				Apply:      deploymentDef("apply last"),
			},
		},
	}); err != nil {
		t.Error(err)
	}

	expected := []command{
		command{"delete", "delete first"},
		command{"apply", "apply last"},
	}
	if !reflect.DeepEqual(expected, mock.commands) {
		t.Errorf("expected commands:\n%#v\ngot:\n%#v", expected, mock.commands)
	}
}

// Test that getting an error in the middle of an action records the
// error, and skips to the next action.
func TestSkipOnError(t *testing.T) {
	kube, mock := setup(t)
	mock.deleteErr = errors.New("create failed")

	def := cluster.SyncDef{
		Actions: []cluster.SyncAction{
			cluster.SyncAction{
				ResourceID: "fail in middle",
				Delete:     deploymentDef("should fail"),
				Apply:      deploymentDef("skipped"),
			},
			cluster.SyncAction{
				ResourceID: "proceed",
				Apply:      deploymentDef("apply works"),
			},
		},
	}

	err := kube.Sync(def)
	switch err := err.(type) {
	case cluster.SyncError:
		if _, ok := err["fail in middle"]; !ok {
			t.Errorf("expected error for failing resource %q, but got %#v", "fail in middle", err)
		}
	default:
		t.Errorf("expected sync error, got %#v", err)
	}

	expected := []command{
		command{"delete", "should fail"},
		// skip to next resource after failure
		command{"apply", "apply works"},
	}
	if !reflect.DeepEqual(expected, mock.commands) {
		t.Errorf("expected commands:\n%#v\ngot:\n%#v", expected, mock.commands)
	}
}
