package kubernetes

import (
	"errors"
	"os"
	"os/exec"

	"github.com/go-kit/kit/log"
	"k8s.io/kubernetes/pkg/client/restclient"
	k8sclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/weaveworks/fluxy/flux"
	"github.com/weaveworks/fluxy/flux/platform"
)

// Cluster is a handle to a Kubernetes API server.
// (Typically, this code is deployed into the same cluster.)
// It collects behaviors that may be performed against Kubernetes.
type Cluster struct {
	config  *restclient.Config
	client  extendedClient
	kubectl string
	logger  log.Logger
}

type extendedClient struct {
	*k8sclient.Client
	*k8sclient.ExtensionsClient
}

// NewCluster returns a usable cluster. kubectl should be the path of a working
// kubectl binary.
func NewCluster(config *restclient.Config, kubectl string, logger log.Logger) (*Cluster, error) {
	client, err := k8sclient.New(config)
	if err != nil {
		return nil, err
	}
	extclient, err := k8sclient.NewExtensions(config)
	if err != nil {
		return nil, err
	}

	if kubectl == "" {
		kubectl, err = exec.LookPath("kubectl")
		if err != nil {
			return nil, err
		}
	} else {
		if _, err := os.Stat(kubectl); err != nil {
			return nil, err
		}
	}
	logger.Log("kubectl", kubectl)

	return &Cluster{
		config:  config,
		client:  extendedClient{client, extclient},
		kubectl: kubectl,
		logger:  logger,
	}, nil
}

// Namespaces returns the active namespaces on the cluster.
func (c *Cluster) Namespaces() ([]string, error) {
	return nil, errors.New("not implemented")
}

// Services returns all platform services in a given namespace.
func (c *Cluster) Services(namespace string) ([]platform.Service, error) {
	return nil, errors.New("not implemented")
}

// Service returns a platform service matching the service ID, if one exists.
func (c *Cluster) Service(serviceID flux.ServiceID) (platform.Service, error) {
	return platform.Service{}, errors.New("not implemented")
}

// ContainersFor returns a list of container names with the image specified to
// run in that container, for a particular service. This is useful to see which
// images a particular service is presently running, to judge whether a release
// is needed.
func (c *Cluster) ContainersFor(serviceID flux.ServiceID) ([]platform.Container, error) {
	return nil, errors.New("not implemented")
}

// Release performs a update of the service, from whatever it is currently, to
// what is described by the new resource, which can be a replication controller
// or deployment.
//
// Release assumes there is a one-to-one mapping between services and
// replication controllers or deployments. It blocks until the update is
// complete. It invokes `kubectl` in a seperate process, and assumes kubectl is
// in the PATH. All of this can be improved.
func (c *Cluster) Release(serviceID flux.ServiceID, newDefinition []byte) error {
	return errors.New("not implemented")
}
