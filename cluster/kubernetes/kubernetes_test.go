package kubernetes

// Test that the implementation of platform wrt Kubernetes is
// adequate. Starting with Sync.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/go-kit/kit/log"
	discovery "k8s.io/client-go/1.5/discovery"
	v1alpha1apps "k8s.io/client-go/1.5/kubernetes/typed/apps/v1alpha1"
	v1beta1authentication "k8s.io/client-go/1.5/kubernetes/typed/authentication/v1beta1"
	v1beta1authorization "k8s.io/client-go/1.5/kubernetes/typed/authorization/v1beta1"
	v1autoscaling "k8s.io/client-go/1.5/kubernetes/typed/autoscaling/v1"
	v1batch "k8s.io/client-go/1.5/kubernetes/typed/batch/v1"
	v1alpha1certificates "k8s.io/client-go/1.5/kubernetes/typed/certificates/v1alpha1"
	v1core "k8s.io/client-go/1.5/kubernetes/typed/core/v1"
	v1beta1extensions "k8s.io/client-go/1.5/kubernetes/typed/extensions/v1beta1"
	v1alpha1policy "k8s.io/client-go/1.5/kubernetes/typed/policy/v1alpha1"
	v1alpha1rbac "k8s.io/client-go/1.5/kubernetes/typed/rbac/v1alpha1"
	v1beta1storage "k8s.io/client-go/1.5/kubernetes/typed/storage/v1beta1"

	"github.com/weaveworks/flux/cluster"
)

type command struct {
	action string
	def    string
}

type mockClientset struct {
}

func (m *mockClientset) Discovery() discovery.DiscoveryInterface {
	return nil
}

func (m *mockClientset) Core() v1core.CoreInterface {
	return nil
}

func (m *mockClientset) Apps() v1alpha1apps.AppsInterface {
	return nil
}

func (m *mockClientset) Authentication() v1beta1authentication.AuthenticationInterface {
	return nil
}

func (m *mockClientset) Authorization() v1beta1authorization.AuthorizationInterface {
	return nil
}

func (m *mockClientset) Autoscaling() v1autoscaling.AutoscalingInterface {
	return nil
}

func (m *mockClientset) Batch() v1batch.BatchInterface {
	return nil
}

func (m *mockClientset) Certificates() v1alpha1certificates.CertificatesInterface {
	return nil
}

func (m *mockClientset) Extensions() v1beta1extensions.ExtensionsInterface {
	return nil
}

func (m *mockClientset) Policy() v1alpha1policy.PolicyInterface {
	return nil
}

func (m *mockClientset) Rbac() v1alpha1rbac.RbacInterface {
	return nil
}

func (m *mockClientset) Storage() v1beta1storage.StorageInterface {
	return nil
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
	clientset := &mockClientset{}
	applier := &mockApplier{}
	kube, err := NewCluster(clientset, applier, nil, log.NewNopLogger())
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
