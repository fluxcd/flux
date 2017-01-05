// Package kubernetes provides abstractions for the Kubernetes platform. At the
// moment, Kubernetes is the only supported platform, so we are directly
// returning Kubernetes objects. As we add more platforms, we will create
// abstractions and common data types in package platform.
package kubernetes

import (
	"os"
	"os/exec"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/kubernetes/pkg/api"
	apiext "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/restclient"
	k8sclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
)

type extendedClient struct {
	*k8sclient.Client
	*k8sclient.ExtensionsClient
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

type applyExecFunc func(*Cluster, log.Logger) error

type apply struct {
	exec    applyExecFunc
	summary string
}

// Cluster is a handle to a Kubernetes API server.
// (Typically, this code is deployed into the same cluster.)
type Cluster struct {
	config  *restclient.Config
	client  extendedClient
	kubectl string
	status  *statusMap
	actionc chan func()
	version string // string response for the version command.
	logger  log.Logger
}

// NewCluster returns a usable cluster. Host should be of the form
// "http://hostname:8080".
func NewCluster(config *restclient.Config, kubectl, version string, logger log.Logger) (*Cluster, error) {
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

	c := &Cluster{
		config:  config,
		client:  extendedClient{client, extclient},
		kubectl: kubectl,
		status:  newStatusMap(),
		actionc: make(chan func()),
		version: version,
		logger:  logger,
	}
	go c.loop()
	return c, nil
}

// Stop terminates the goroutine that serializes and executes requests against
// the cluster. A stopped cluster cannot be restarted.
func (c *Cluster) Stop() {
	close(c.actionc)
}

func (c *Cluster) loop() {
	for f := range c.actionc {
		f()
	}
}

// --- platform API

// SomeServices returns the services named, missing out any that don't
// exist in the cluster. They do not necessarily have to be returned
// in the order requested.
func (c *Cluster) SomeServices(ids []flux.ServiceID) (res []platform.Service, err error) {
	namespacedServices := map[string][]string{}
	for _, id := range ids {
		ns, name := id.Components()
		namespacedServices[ns] = append(namespacedServices[ns], name)
	}

	for ns, names := range namespacedServices {
		services := c.client.Services(ns)
		controllers, err := c.podControllersInNamespace(ns)
		if err != nil {
			return nil, errors.Wrapf(err, "finding pod controllers for namespace %s", ns)
		}
		for _, name := range names {
			service, err := services.Get(name)
			if err != nil {
				return nil, errors.Wrapf(err, "finding service %s among services for namespace %s", name, ns)
			}

			res = append(res, c.makeService(ns, service, controllers))
		}
	}
	return res, nil
}

// AllServices returns all services matching the criteria; that is, in
// the namespace (or any namespace if that argument is empty), and not
// in the `ignore` set given.
func (c *Cluster) AllServices(namespace string, ignore flux.ServiceIDSet) (res []platform.Service, err error) {
	namespaces := []string{}
	if namespace == "" {
		list, err := c.client.Namespaces().List(api.ListOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "getting namespaces")
		}
		for _, ns := range list.Items {
			namespaces = append(namespaces, ns.Name)
		}
	} else {
		namespaces = []string{namespace}
	}

	for _, ns := range namespaces {
		controllers, err := c.podControllersInNamespace(ns)
		if err != nil {
			return nil, errors.Wrapf(err, "getting pod controllers for namespace %s", ns)
		}

		list, err := c.client.Services(ns).List(api.ListOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "getting services for namespace %s", ns)
		}

		for _, service := range list.Items {
			if !ignore.Contains(flux.MakeServiceID(ns, service.Name)) {
				res = append(res, c.makeService(ns, &service, controllers))
			}
		}
	}
	return res, nil
}

func (c *Cluster) makeService(ns string, service *api.Service, controllers []podController) platform.Service {
	id := flux.MakeServiceID(ns, service.Name)
	status, _ := c.status.getApplyProgress(id)
	return platform.Service{
		ID:         id,
		IP:         service.Spec.ClusterIP,
		Metadata:   metadataForService(service),
		Containers: containersOrExcuse(service, controllers),
		Status:     status,
	}
}

func metadataForService(s *api.Service) map[string]string {
	return map[string]string{
		"created_at":       s.CreationTimestamp.String(),
		"resource_version": s.ResourceVersion,
		"uid":              string(s.UID),
		"type":             string(s.Spec.Type),
	}
}

func (c *Cluster) podControllersInNamespace(namespace string) (res []podController, err error) {
	deploylist, err := c.client.Deployments(namespace).List(api.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "collecting deployments")
	}
	for i := range deploylist.Items {
		res = append(res, podController{Deployment: &deploylist.Items[i]})
	}

	rclist, err := c.client.ReplicationControllers(namespace).List(api.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "collecting replication controllers")
	}
	for i := range rclist.Items {
		res = append(res, podController{ReplicationController: &rclist.Items[i]})
	}

	return res, nil
}

// Find the pod controller (deployment or replication controller) that matches the service
func matchController(service *api.Service, controllers []podController) (podController, error) {
	selector := service.Spec.Selector
	if len(selector) == 0 {
		return podController{}, platform.ErrEmptySelector
	}

	var matching []podController
	for _, c := range controllers {
		if c.matchedBy(selector) {
			matching = append(matching, c)
		}
	}
	switch len(matching) {
	case 1:
		return matching[0], nil
	case 0:
		return podController{}, platform.ErrNoMatching
	default:
		return podController{}, platform.ErrMultipleMatching
	}
}

func containersOrExcuse(service *api.Service, controllers []podController) platform.ContainersOrExcuse {
	pc, err := matchController(service, controllers)
	if err != nil {
		return platform.ContainersOrExcuse{Excuse: err.Error()}
	}
	return platform.ContainersOrExcuse{Containers: pc.templateContainers()}
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

func (p podController) templateContainers() (res []platform.Container) {
	var apiContainers []api.Container
	if p.Deployment != nil {
		apiContainers = p.Deployment.Spec.Template.Spec.Containers
	} else if p.ReplicationController != nil {
		apiContainers = p.ReplicationController.Spec.Template.Spec.Containers
	}

	for _, c := range apiContainers {
		res = append(res, platform.Container{Name: c.Name, Image: c.Image})
	}
	return res
}

func (p podController) templateLabels() map[string]string {
	if p.Deployment != nil {
		return p.Deployment.Spec.Template.Labels
	} else if p.ReplicationController != nil {
		return p.ReplicationController.Spec.Template.Labels
	}
	return nil
}

func (p podController) matchedBy(selector map[string]string) bool {
	// For each key=value pair in the service spec, check if the RC
	// annotates its pods in the same way. If any rule fails, the RC is
	// not a match. If all rules pass, the RC is a match.
	labels := p.templateLabels()
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// Apply applies a new set of ServiceDefinition. If all definitions succeed,
// Apply returns a nil error. If any definitions fail, Apply returns an error
// of type ApplyError, which can be inspected for more detailed information.
// Applies are serialized per cluster.
//
// Apply assumes there is a one-to-one mapping between services and replication
// controllers or deployments; this can be improved. Apply blocks until an
// update is complete; this can be improved. Apply invokes `kubectl
// rolling-update` or `kubectl apply` in a seperate process, and assumes
// kubectl is in the PATH; this can be improved.
func (c *Cluster) Apply(defs []platform.ServiceDefinition) error {
	errc := make(chan error)
	c.actionc <- func() {
		namespacedDefs := map[string][]platform.ServiceDefinition{}
		for _, def := range defs {
			ns, _ := def.ServiceID.Components()
			namespacedDefs[ns] = append(namespacedDefs[ns], def)
		}

		applyErr := platform.ApplyError{}
		for namespace, defs := range namespacedDefs {
			services := c.client.Services(namespace)

			controllers, err := c.podControllersInNamespace(namespace)
			if err != nil {
				err = errors.Wrapf(err, "getting pod controllers for namespace %s", namespace)
				for _, def := range defs {
					applyErr[def.ServiceID] = err
				}
				continue
			}

			for _, def := range defs {
				newDef, err := definitionObj(def.NewDefinition)
				if err != nil {
					applyErr[def.ServiceID] = errors.Wrap(err, "reading definition")
					continue
				}

				_, serviceName := def.ServiceID.Components()
				service, err := services.Get(serviceName)
				if err != nil {
					applyErr[def.ServiceID] = errors.Wrap(err, "getting service")
					continue
				}

				controller, err := matchController(service, controllers)
				if err != nil {
					applyErr[def.ServiceID] = errors.Wrap(err, "getting pod controller")
					continue
				}

				plan, err := controller.newApply(newDef)
				if err != nil {
					applyErr[def.ServiceID] = errors.Wrap(err, "creating release")
					continue
				}

				c.status.startApply(def.ServiceID, plan)
				defer c.status.endApply(def.ServiceID)

				logger := log.NewContext(c.logger).With("method", "Apply", "namespace", namespace, "service", serviceName)
				if err = plan.exec(c, logger); err != nil {
					applyErr[def.ServiceID] = errors.Wrapf(err, "applying definition to %s", def.ServiceID)
					continue
				}
			}
		}
		if len(applyErr) > 0 {
			errc <- applyErr
			return
		}
		errc <- nil
	}
	return <-errc
}

func definitionObj(bytes []byte) (*apiObject, error) {
	obj := apiObject{bytes: bytes}
	return &obj, yaml.Unmarshal(bytes, &obj)
}

func (c *Cluster) Ping() error {
	_, err := c.client.ServerVersion()
	return err
}

func (c *Cluster) Version() (string, error) {
	return c.version, nil
}

// --- end platform API

type statusMap struct {
	inProgress map[flux.ServiceID]*apply
	mx         sync.RWMutex
}

func newStatusMap() *statusMap {
	return &statusMap{
		inProgress: make(map[flux.ServiceID]*apply),
	}
}

func (m *statusMap) startApply(s flux.ServiceID, a *apply) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.inProgress[s] = a
}

func (m *statusMap) getApplyProgress(s flux.ServiceID) (string, bool) {
	m.mx.RLock()
	defer m.mx.RUnlock()
	if a, ok := m.inProgress[s]; ok {
		return a.summary, true
	}
	return "", false
}

func (m *statusMap) endApply(s flux.ServiceID) {
	m.mx.Lock()
	defer m.mx.Unlock()
	delete(m.inProgress, s)
}
