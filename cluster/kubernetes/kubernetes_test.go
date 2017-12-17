package kubernetes

// Test that the implementation of platform wrt Kubernetes is
// adequate. Starting with Sync.

import (
	"reflect"
	"testing"

	"github.com/go-kit/kit/log"
	discovery "k8s.io/client-go/discovery"
	admissionregistrationv1alpha1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1alpha1"
	appsv1beta1 "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
	authenticationv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authenticationv1beta1 "k8s.io/client-go/kubernetes/typed/authentication/v1beta1"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	authorizationv1beta1 "k8s.io/client-go/kubernetes/typed/authorization/v1beta1"
	autoscalingv1 "k8s.io/client-go/kubernetes/typed/autoscaling/v1"
	autoscalingv2alpha1 "k8s.io/client-go/kubernetes/typed/autoscaling/v2alpha1"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	batchv2alpha1 "k8s.io/client-go/kubernetes/typed/batch/v2alpha1"
	certificatesv1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	extensionsv1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	networkingv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	policyv1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	rbacv1alpha1 "k8s.io/client-go/kubernetes/typed/rbac/v1alpha1"
	rbacv1beta1 "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
	settingsv1alpha1 "k8s.io/client-go/kubernetes/typed/settings/v1alpha1"
	storagev1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	storagev1beta1 "k8s.io/client-go/kubernetes/typed/storage/v1beta1"

	"github.com/weaveworks/flux/cluster"
)

type mockClientset struct {
}

func (m *mockClientset) Discovery() discovery.DiscoveryInterface {
	return nil
}

func (m *mockClientset) AdmissionregistrationV1alpha1() admissionregistrationv1alpha1.AdmissionregistrationV1alpha1Interface {
	return nil
}

func (m *mockClientset) Admissionregistration() admissionregistrationv1alpha1.AdmissionregistrationV1alpha1Interface {
	return nil
}

func (m *mockClientset) CoreV1() corev1.CoreV1Interface {
	return nil
}

func (m *mockClientset) Core() corev1.CoreV1Interface {
	return nil
}

func (m *mockClientset) AppsV1beta1() appsv1beta1.AppsV1beta1Interface {
	return nil
}

func (m *mockClientset) Apps() appsv1beta1.AppsV1beta1Interface {
	return nil
}

func (m *mockClientset) AuthenticationV1() authenticationv1.AuthenticationV1Interface {
	return nil
}

func (m *mockClientset) Authentication() authenticationv1.AuthenticationV1Interface {
	return nil
}

func (m *mockClientset) AuthenticationV1beta1() authenticationv1beta1.AuthenticationV1beta1Interface {
	return nil
}

func (m *mockClientset) AuthorizationV1() authorizationv1.AuthorizationV1Interface {
	return nil
}

func (m *mockClientset) Authorization() authorizationv1.AuthorizationV1Interface {
	return nil
}

func (m *mockClientset) AuthorizationV1beta1() authorizationv1beta1.AuthorizationV1beta1Interface {
	return nil
}

func (m *mockClientset) AutoscalingV1() autoscalingv1.AutoscalingV1Interface {
	return nil
}

func (m *mockClientset) Autoscaling() autoscalingv1.AutoscalingV1Interface {
	return nil
}

func (m *mockClientset) AutoscalingV2alpha1() autoscalingv2alpha1.AutoscalingV2alpha1Interface {
	return nil
}

func (m *mockClientset) BatchV1() batchv1.BatchV1Interface {
	return nil
}

func (m *mockClientset) Batch() batchv1.BatchV1Interface {
	return nil
}

func (m *mockClientset) BatchV2alpha1() batchv2alpha1.BatchV2alpha1Interface {
	return nil
}

func (m *mockClientset) CertificatesV1beta1() certificatesv1beta1.CertificatesV1beta1Interface {
	return nil
}

func (m *mockClientset) Certificates() certificatesv1beta1.CertificatesV1beta1Interface {
	return nil
}

func (m *mockClientset) ExtensionsV1beta1() extensionsv1beta1.ExtensionsV1beta1Interface {
	return nil
}

func (m *mockClientset) Extensions() extensionsv1beta1.ExtensionsV1beta1Interface {
	return nil
}

func (m *mockClientset) NetworkingV1() networkingv1.NetworkingV1Interface {
	return nil
}

func (m *mockClientset) Networking() networkingv1.NetworkingV1Interface {
	return nil
}

func (m *mockClientset) PolicyV1beta1() policyv1beta1.PolicyV1beta1Interface {
	return nil
}

func (m *mockClientset) Policy() policyv1beta1.PolicyV1beta1Interface {
	return nil
}

func (m *mockClientset) RbacV1beta1() rbacv1beta1.RbacV1beta1Interface {
	return nil
}

func (m *mockClientset) Rbac() rbacv1beta1.RbacV1beta1Interface {
	return nil
}

func (m *mockClientset) RbacV1alpha1() rbacv1alpha1.RbacV1alpha1Interface {
	return nil
}

func (m *mockClientset) SettingsV1alpha1() settingsv1alpha1.SettingsV1alpha1Interface {
	return nil
}

func (m *mockClientset) Settings() settingsv1alpha1.SettingsV1alpha1Interface {
	return nil
}

func (m *mockClientset) StorageV1beta1() storagev1beta1.StorageV1beta1Interface {
	return nil
}

func (m *mockClientset) StorageV1() storagev1.StorageV1Interface {
	return nil
}

func (m *mockClientset) Storage() storagev1.StorageV1Interface {
	return nil
}

type mockApplier struct {
	commands  []string
	applyErr  error
	createErr error
	deleteErr error

	changeSet
}

func (m *mockApplier) apply(_ log.Logger, _ []byte) error {
	m.commands = append(m.commands, "apply")
	return m.applyErr
}

func (m *mockApplier) delete(_ log.Logger, _ []byte) error {
	m.commands = append(m.commands, "delete")
	return m.deleteErr
}

func (m *mockApplier) execute(_ log.Logger, errs cluster.SyncError) error {
	if len(m.deleteObjs) > 0 {
		if err := m.delete(nil, nil); err != nil {
			return err
		}
	}
	if len(m.applyObjs) > 0 {
		if err := m.apply(nil, nil); err != nil {
			return err
		}
	}
	if len(errs) != 0 {
		return errs
	}
	return nil
}

func deploymentDef(name string) []byte {
	return []byte(`---
kind: Deployment
metadata:
  name: ` + name)
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

	expected := []string{
		"delete",
		"apply",
	}
	if !reflect.DeepEqual(expected, mock.commands) {
		t.Errorf("expected commands:\n%#v\ngot:\n%#v", expected, mock.commands)
	}
}
