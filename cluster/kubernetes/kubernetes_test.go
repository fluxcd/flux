package kubernetes

// Test that the implementation of platform wrt Kubernetes is
// adequate. Starting with Sync.

import (
	"encoding/base64"
	"errors"
	"reflect"
	"testing"

	"github.com/go-kit/kit/log"
	"k8s.io/client-go/1.5/discovery"
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
	"k8s.io/client-go/1.5/pkg/api/v1"

	"fmt"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/registry"
	"k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"
	"os"
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
	kube, err := NewCluster(clientset, applier, nil, nil, nil, log.NewNopLogger())
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

const (
	ns1      = "ns1"
	ns2      = "ns2"
	mysecret = "mysecret"
)

var (
	svcName1 = "svcID1"
	svcName2 = "svcID2"
	svcName3 = "svcID3"
	svcID1   = flux.ServiceID(ns1 + "/" + svcName1)
	svcID2   = flux.ServiceID(ns1 + "/" + svcName2)
	svcID3   = flux.ServiceID(ns2 + "/" + svcName3)
	svc1     = cluster.Service{
		ID: svcID1,
		Containers: cluster.ContainersOrExcuse{
			Containers: []cluster.Container{
				{
					Name:  "alpine",
					Image: "alpine:3.6",
				},
			},
		},
	}
	svc2 = cluster.Service{
		ID: svcID2,
		Containers: cluster.ContainersOrExcuse{
			Containers: []cluster.Container{
				{
					Name:  "custom",
					Image: "localhost:5000/my/image:latest",
				},
			},
		},
	}
	svc3 = cluster.Service{
		ID: svcID3,
	}
	host        = "hub.docker.io"
	tmpl string = `
    {
        "auths": {
            %q: {"auth": %q}
        }
    }`
	okCreds                             string = base64.StdEncoding.EncodeToString([]byte("user:pass"))
	stringCreds                                = fmt.Sprintf(tmpl, host, okCreds)
	testCredentials, testCredentialsErr        = registry.ParseCredentials([]byte(stringCreds))
)

type mockFluxAdapter struct {
}

func (*mockFluxAdapter) Services(namespace string) ([]cluster.Service, error) {
	if namespace == ns1 {
		return []cluster.Service{svc1, svc2}, nil
	} else if namespace == ns2 {
		return []cluster.Service{svc3}, nil
	}
	return []cluster.Service{svc1, svc2, svc3}, nil
}

func (*mockFluxAdapter) ImageCredentials(service cluster.Service) (registry.Credentials, error) {
	switch service.ID.String() {
	case svc1.ID.String():
		return testCredentials, nil
	default:
		return registry.NoCredentials(), nil
	}
}

type mockKubeAPI struct{}

func (*mockKubeAPI) KubeServices(namespace string) (*v1.ServiceList, error) {
	if namespace == ns1 {
		return &v1.ServiceList{
			Items: []v1.Service{
				{
					ObjectMeta: v1.ObjectMeta{
						Namespace: ns1,
						Name:      svcName1,
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Namespace: ns1,
						Name:      svcName2,
					},
				},
			},
		}, nil
	} else {
		return &v1.ServiceList{
			Items: []v1.Service{
				{
					ObjectMeta: v1.ObjectMeta{
						Namespace: ns2,
						Name:      svcName3,
					},
				},
			},
		}, nil
	}
}

func (*mockKubeAPI) KubeService(ns, svc string) (*v1.Service, error) {
	if ns == ns1 && svc == svcName1 {
		return &v1.Service{
			ObjectMeta: v1.ObjectMeta{
				Namespace: ns1,
				Name:      svcName1,
			},
		}, nil
	}
	return &v1.Service{}, errors.New("No service found")
}

func (*mockKubeAPI) KubeControllers(svc *v1.Service) ([]podController, error) {
	if svc.Name == svcName1 {
		return []podController{
			{
				Deployment: &v1beta1.Deployment{
					Spec: v1beta1.DeploymentSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test",
										Image: "alpine:3.6",
									},
								},
								ImagePullSecrets: []v1.LocalObjectReference{
									{
										Name: mysecret,
									},
								},
							},
						},
					},
				},
			},
		}, nil
	} else {
		return []podController{}, nil
	}
}

func (*mockKubeAPI) KubeSecrets(ns, secret string) (*v1.Secret, error) {
	if ns == ns1 && secret == mysecret {
		return &v1.Secret{
			Type: v1.SecretTypeDockercfg,
			Data: map[string][]byte{
				v1.DockerConfigKey: []byte(stringCreds),
			},
		}, nil
	}
	return &v1.Secret{}, errors.New("Secret not found")
}

func (*mockKubeAPI) KubeNamespaces() (*v1.NamespaceList, error) {
	return &v1.NamespaceList{
		Items: []v1.Namespace{
			{
				ObjectMeta: v1.ObjectMeta{
					Name: ns1,
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: ns2,
				},
			},
		},
	}, nil
}

func TestKubernetes_ImagesToFetch(t *testing.T) {
	if testCredentialsErr != nil {
		t.Fatal("Credentials weren't parsed properly", testCredentialsErr)
	}
	mock := mockFluxAdapter{}

	f := NewImageFetcher(&mock, log.NewLogfmtLogger(os.Stderr))

	// Call images to fetch
	imageCreds := f.ImagesToFetch()

	// Make sure it is what we expect
	if len(imageCreds) != 2 {
		t.Fatal("Incorrect number of image credentials", len(imageCreds))
	}

	// Find service with credentials
	var credentials registry.Credentials
	var image flux.ImageID
	for k, imCred := range imageCreds {
		if len(imCred.Hosts()) > 0 {
			image = k
			credentials = imCred
			break
		}
	}

	if len(credentials.Hosts()) != 1 {
		t.Fatal("Wrong number of hosts returned", len(credentials.Hosts()))
	} else if image.Image != "alpine" {
		t.Fatal("Wrong image returned", image.Image)
	} else if !reflect.DeepEqual(credentials.Hosts(), []string{host}) {
		t.Fatal("Wrong credentials hosts returned", credentials.Hosts())
	}
}

func TestKubernetes_FluxServiceCredentials(t *testing.T) {
	mock := mockKubeAPI{}
	sc := NewKubeFluxAdapter(&mock, log.NewLogfmtLogger(os.Stderr))

	// All namespaces
	svcs, err := sc.Services("")
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 3 {
		t.Fatal("Expected three services. Got", len(svcs))
	}
	numNamespaces := make(map[string]string)
	for _, v := range svcs {
		ns, _ := v.ID.Components()
		numNamespaces[ns] = ""
	}
	if len(numNamespaces) != 2 {
		t.Fatal("Expecting services to live within two namespaces. Got", len(numNamespaces))
	}

	// Specific namespace
	svcs, err = sc.Services(ns1)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 2 {
		t.Fatal("Expected three services. Got", len(svcs))
	}
	numNamespaces = make(map[string]string)
	for _, v := range svcs {
		ns, _ := v.ID.Components()
		numNamespaces[ns] = ""
	}
	if len(numNamespaces) != 1 {
		t.Fatal("Expecting services to live within two namespaces. Got", len(numNamespaces))
	}

	creds, err := sc.ImageCredentials(svc1)
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(creds.Hosts()) != 1 {
		t.Fatal("Expecting single host with credentials. Got", len(creds.Hosts()))
	}
	if creds.Hosts()[0] != host {
		t.Fatal("Expected host of %q but got %q", host, creds.Hosts()[0])
	}
}

func TestKubernetes_AllServices(t *testing.T) {
	mock := mockFluxAdapter{}
	c := Cluster{
		fluxAdapter: &mock,
	}
	// All namespaces
	svcs, err := c.AllServices("")
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 3 {
		t.Fatal("Expected three services. Got", len(svcs))
	}
	numNamespaces := make(map[string]string)
	for _, v := range svcs {
		ns, _ := v.ID.Components()
		numNamespaces[ns] = ""
	}
	if len(numNamespaces) != 2 {
		t.Fatal("Expecting services to live within two namespaces. Got", len(numNamespaces))
	}

	// Specific namespace
	svcs, err = c.AllServices(ns1)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 2 {
		t.Fatal("Expected three services. Got", len(svcs))
	}
	numNamespaces = make(map[string]string)
	for _, v := range svcs {
		ns, _ := v.ID.Components()
		numNamespaces[ns] = ""
	}
	if len(numNamespaces) != 1 {
		t.Fatal("Expecting services to live within two namespaces. Got", len(numNamespaces))
	}
}

func TestKubernetes_SomeServices(t *testing.T) {
	mock := mockFluxAdapter{}
	c := Cluster{
		fluxAdapter: &mock,
	}
	// One ID
	svcs, err := c.SomeServices([]flux.ServiceID{svcID1})
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 1 {
		t.Fatal("Expected three services. Got", len(svcs))
	}
	if svcs[0].ID.String() != svcID1.String() {
		t.Fatal("Did not return expected service id", svcs[0].ID.String())
	}
	if len(svcs[0].Containers.Containers) == 0 {
		t.Fatal("Service didn't return any containers, even though they are mocked")
	}

	// Multiple services
	svcs, err = c.SomeServices([]flux.ServiceID{svcID1, svcID3})
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 2 {
		t.Fatal("Expected three services. Got", len(svcs))
	}
	numNamespaces := make(map[string]string)
	for _, v := range svcs {
		ns, _ := v.ID.Components()
		numNamespaces[ns] = ""
	}
	if len(numNamespaces) != 2 {
		t.Fatal("Expecting services to live within two namespaces. Got", len(numNamespaces))
	}
}
