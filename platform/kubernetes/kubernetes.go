// Package kubernetes provides abstractions for the Kubernetes platform. At the
// moment, Kubernetes is the only supported platform, so we are directly
// returning Kubernetes objects. As we add more platforms, we will create
// abstractions and common data types in package platform.
package kubernetes

import (
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/kubernetes/pkg/api"
	k8serrors "k8s.io/kubernetes/pkg/api/errors"
	apiext "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/restclient"
	k8sclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/weaveworks/fluxy/platform"
)

type extendedClient struct {
	*k8sclient.Client
	*k8sclient.ExtensionsClient
}

type namespacedService struct {
	namespace string
	service   string
}

type apiObject struct {
	bytes    []byte
	Version  string `yaml:"apiVersion"`
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

type regradeExecFunc func(*Cluster, log.Logger) error

type regrade struct {
	exec    regradeExecFunc
	summary string
}

// Cluster is a handle to a Kubernetes API server.
// (Typically, this code is deployed into the same cluster.)
type Cluster struct {
	config  *restclient.Config
	client  extendedClient
	kubectl string
	status  *statusMap
	logger  log.Logger
}

// NewCluster returns a usable cluster. Host should be of the form
// "http://hostname:8080".
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
		status:  newStatusMap(),
		logger:  logger,
	}, nil
}

// Namespaces returns the set of available namespaces on the platform.
func (c *Cluster) Namespaces() ([]string, error) {
	list, err := c.client.Namespaces().List(api.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "fetching namespaces")
	}
	res := make([]string, len(list.Items))
	for i := range list.Items {
		res[i] = list.Items[i].Name
	}
	return res, nil
}

// Service returns the platform.Service representation of the named service.
func (c *Cluster) Service(namespace, service string) (platform.Service, error) {
	apiService, err := c.service(namespace, service)
	if err != nil {
		if statusErr, ok := err.(*k8serrors.StatusError); ok && statusErr.ErrStatus.Code == http.StatusNotFound { // le sigh
			return platform.Service{}, platform.ErrNoMatchingService
		}
		return platform.Service{}, errors.Wrap(err, "fetching service "+namespace+"/"+service)
	}
	return c.makePlatformService(apiService), nil
}

// Services returns the set of services currently active on the platform in the
// given namespace. Maybe it makes sense to move the namespace to the
// constructor? Depends on how it will be used. For now it is here.
//
// The user is expected to list services, and then choose the one that will
// receive a release. Releases operate on replication controllers, not services.
// For now, we make a simplifying assumption that there is a one-to-one mapping
// between services and replication controllers.
func (c *Cluster) Services(namespace string) ([]platform.Service, error) {
	apiServices, err := c.services(namespace)
	if err != nil {
		return nil, errors.Wrap(err, "fetching services for namespace "+namespace)
	}
	return c.makePlatformServices(apiServices), nil
}

func (c *Cluster) service(namespace, service string) (res api.Service, err error) {
	apiService, err := c.client.Services(namespace).Get(service)
	if err != nil {
		return api.Service{}, err
	}
	return *apiService, nil
}

func (c *Cluster) services(namespace string) (res []api.Service, err error) {
	list, err := c.client.Services(namespace).List(api.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func definitionObj(bytes []byte) (*apiObject, error) {
	obj := apiObject{bytes: bytes}
	return &obj, yaml.Unmarshal(bytes, &obj)
}

// Regrade performs an update of the service, from whatever it is currently, to
// what is described by the new resource, which can be a replication controller
// or deployment.
//
// Regrade assumes there is a one-to-one mapping between services and
// replication controllers or deployments; this can be
// improved. Regrade blocks until a rolling update is complete; this
// can be improved. Regrade invokes `kubectl rolling-update` or
// `kubectl apply` in a seperate process, and assumes kubectl is in
// the PATH; this can be improved.
func (c *Cluster) Regrade(namespace, serviceName string, newDefinition []byte) error {
	newDef, err := definitionObj(newDefinition)
	if err != nil {
		return errors.Wrap(err, "reading definition")
	}

	pc, err := c.podControllerFor(namespace, serviceName)
	if err != nil {
		return err
	}

	plan, err := pc.newRegrade(newDef)
	if err != nil {
		return err
	}

	ns := namespacedService{namespace, serviceName}
	c.status.startRegrade(ns, plan)
	defer c.status.endRegrade(ns)

	logger := log.NewContext(c.logger).With("method", "Release", "namespace", namespace, "service", serviceName)
	if err = plan.exec(c, logger); err != nil {
		return errors.Wrap(err, "releasing "+namespace+"/"+serviceName)
	}
	return nil
}

type statusMap struct {
	inProgress map[namespacedService]*regrade
	mx         sync.RWMutex
}

func newStatusMap() *statusMap {
	return &statusMap{
		inProgress: make(map[namespacedService]*regrade),
	}
}

func (m *statusMap) startRegrade(ns namespacedService, r *regrade) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.inProgress[ns] = r
}

func (m *statusMap) getRegradeProgress(ns namespacedService) (string, bool) {
	m.mx.RLock()
	defer m.mx.RUnlock()
	if r, found := m.inProgress[ns]; found {
		return r.summary, true
	}
	return "", false
}

func (m *statusMap) endRegrade(ns namespacedService) {
	m.mx.Lock()
	defer m.mx.Unlock()
	delete(m.inProgress, ns)
}

// Either a replication controller, a deployment, or neither (both nils).
type podController struct {
	ReplicationController *api.ReplicationController
	Deployment            *apiext.Deployment
}

func (p podController) name() string {
	if p.Deployment != nil {
		return p.Deployment.Name
	} else if p.ReplicationController != nil {
		return p.ReplicationController.Name
	}
	return ""
}

func (p podController) kind() string {
	if p.Deployment != nil {
		return "Deployment"
	} else if p.ReplicationController != nil {
		return "ReplicationController"
	}
	return "unknown"
}

func (p podController) templateContainers() []api.Container {
	if p.Deployment != nil {
		return p.Deployment.Spec.Template.Spec.Containers
	} else if p.ReplicationController != nil {
		return p.ReplicationController.Spec.Template.Spec.Containers
	}
	return nil
}

func (c *Cluster) podControllerFor(namespace, serviceName string) (res podController, err error) {
	res = podController{}

	service, err := c.service(namespace, serviceName)
	if err != nil {
		return res, errors.Wrap(err, "fetching service "+namespace+"/"+serviceName)
	}

	selector := service.Spec.Selector
	if len(selector) <= 0 {
		return res, platform.ErrServiceHasNoSelector
	}

	// Now, try to find a deployment or replication controller that matches the
	// selector given in the service. The simplifying assumption for the time
	// being is that there's just one of these -- we return an error otherwise.

	// Find a replication controller which produces pods that match that
	// selector. We have to match all of the criteria in the selector, but we
	// don't need a perfect match of all of the replication controller's pod
	// properties.
	rclist, err := c.client.ReplicationControllers(namespace).List(api.ListOptions{})
	if err != nil {
		return res, errors.Wrap(err, "fetching replication controllers for ns "+namespace)
	}
	var rcs []api.ReplicationController
	for _, rc := range rclist.Items {
		match := func() bool {
			// For each key=value pair in the service spec, check if the RC
			// annotates its pods in the same way. If any rule fails, the RC is
			// not a match. If all rules pass, the RC is a match.
			for k, v := range selector {
				labels := rc.Spec.Template.Labels
				if labels[k] != v {
					return false
				}
			}
			return true
		}()
		if match {
			rcs = append(rcs, rc)
		}
	}
	switch len(rcs) {
	case 0:
		break // we can hope to find a deployment
	case 1:
		res.ReplicationController = &rcs[0]
	default:
		return res, platform.ErrMultipleMatching
	}

	// Now do the same work for deployments.
	deplist, err := c.client.Deployments(namespace).List(api.ListOptions{})
	if err != nil {
		return res, errors.Wrap(err, "fetching deployments for ns "+namespace)
	}
	var deps []apiext.Deployment
	for _, d := range deplist.Items {
		match := func() bool {
			// For each key=value pair in the service spec, check if the
			// deployment annotates its pods in the same way. If any rule fails,
			// the deployment is not a match. If all rules pass, the deployment
			// is a match.
			for k, v := range selector {
				labels := d.Spec.Template.Labels
				if labels[k] != v {
					return false
				}
			}
			return true
		}()
		if match {
			deps = append(deps, d)
		}
	}
	switch len(deps) {
	case 0:
		break
	case 1:
		res.Deployment = &deps[0]
	default:
		return res, platform.ErrMultipleMatching
	}

	if res.ReplicationController != nil && res.Deployment != nil {
		return res, platform.ErrMultipleMatching
	}
	if res.ReplicationController == nil && res.Deployment == nil {
		return res, platform.ErrNoMatching
	}
	return res, nil
}

// ContainersFor returns a list of container names with the image
// specified to run in that container, for a particular service. This
// is useful to see which images a particular service is presently
// running, to judge whether a release is needed.
func (c *Cluster) ContainersFor(namespace, serviceName string) (res []platform.Container, err error) {
	pc, err := c.podControllerFor(namespace, serviceName)
	if err != nil {
		return nil, err
	}

	var containers []platform.Container
	for _, container := range pc.templateContainers() {
		containers = append(containers, platform.Container{
			Image: container.Image,
			Name:  container.Name,
		})
	}
	if len(containers) <= 0 {
		return nil, platform.ErrNoMatchingImages
	}
	return containers, nil
}

func (c *Cluster) makePlatformServices(apiServices []api.Service) []platform.Service {
	platformServices := make([]platform.Service, len(apiServices))
	for i, s := range apiServices {
		platformServices[i] = c.makePlatformService(s)
	}
	return platformServices
}

func (c *Cluster) makePlatformService(s api.Service) platform.Service {
	metadata := map[string]string{
		"created_at":       s.CreationTimestamp.String(),
		"resource_version": s.ResourceVersion,
		"uid":              string(s.UID),
		"type":             string(s.Spec.Type),
	}

	var status string
	if summary, found := c.status.getRegradeProgress(namespacedService{s.Namespace, s.Name}); found {
		status = summary
	}

	return platform.Service{
		Name:     s.Name,
		IP:       s.Spec.ClusterIP,
		Metadata: metadata,
		Status:   status,
	}
}
