package kubernetes

// Test that the implementation of platform wrt Kubernetes is
// adequate. Starting with Sync.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/go-kit/kit/log"
	"k8s.io/client-go/1.5/rest"

	"github.com/weaveworks/flux/platform"
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

func (m *mockApplier) Create(logger log.Logger, def []byte) error {
	m.commands = append(m.commands, command{"create", string(def)})
	return m.createErr
}

func (m *mockApplier) Apply(logger log.Logger, def []byte) error {
	m.commands = append(m.commands, command{"apply", string(def)})
	return m.applyErr
}

func (m *mockApplier) Delete(logger log.Logger, def []byte) error {
	m.commands = append(m.commands, command{"delete", string(def)})
	return m.deleteErr
}

// ---

func setup(t *testing.T) (platform.Platform, *mockApplier) {
	restClientConfig := &rest.Config{}
	applier := &mockApplier{}
	kube, err := NewCluster(restClientConfig, applier, "test-version", log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	return kube, applier
}

func TestSyncNop(t *testing.T) {
	kube, mock := setup(t)
	if err := kube.Sync(platform.SyncDef{}); err != nil {
		t.Error(err)
	}
	if len(mock.commands) > 0 {
		t.Errorf("expected no commands run, but got %#v", mock.commands)
	}
}

func TestSyncOrder(t *testing.T) {
	kube, mock := setup(t)
	if err := kube.Sync(platform.SyncDef{
		Actions: []platform.SyncAction{
			platform.SyncAction{
				ResourceID: "foobar",
				Delete:     []byte("delete first"),
				Create:     []byte("create second"),
				Apply:      []byte("apply last"),
			},
		},
	}); err != nil {
		t.Error(err)
	}

	expected := []command{
		command{"delete", "delete first"},
		command{"create", "create second"},
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
	mock.createErr = errors.New("create failed")

	def := platform.SyncDef{
		Actions: []platform.SyncAction{
			platform.SyncAction{
				ResourceID: "fail in middle",
				Delete:     []byte("succeeds"),
				Create:     []byte("should fail"),
				Apply:      []byte("skipped"),
			},
			platform.SyncAction{
				ResourceID: "proceed",
				Apply:      []byte("apply works"),
			},
		},
	}

	err := kube.Sync(def)
	switch err := err.(type) {
	case platform.SyncError:
		if _, ok := err["fail in middle"]; !ok {
			t.Errorf("expected error for failing resource %q, but got %#v", "fail in middle", err)
		}
	default:
		t.Errorf("expected sync error, got %#v", err)
	}

	expected := []command{
		command{"delete", "succeeds"},
		command{"create", "should fail"},
		// skip to next resource after failure
		command{"apply", "apply works"},
	}
	if !reflect.DeepEqual(expected, mock.commands) {
		t.Errorf("expected commands:\n%#v\ngot:\n%#v", expected, mock.commands)
	}
}
