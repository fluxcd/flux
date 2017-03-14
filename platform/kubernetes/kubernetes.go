// Package kubernetes provides abstractions for the Kubernetes platform. At the
// moment, Kubernetes is the only supported platform, so we are directly
// returning Kubernetes objects. As we add more platforms, we will create
// abstractions and common data types in package platform.
package kubernetes

import (
	"bytes"
	"os"
	"os/exec"

	k8syaml "github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	discovery "k8s.io/client-go/1.5/discovery"
	k8sclient "k8s.io/client-go/1.5/kubernetes"
	v1core "k8s.io/client-go/1.5/kubernetes/typed/core/v1"
	v1beta1extensions "k8s.io/client-go/1.5/kubernetes/typed/extensions/v1beta1"
	api "k8s.io/client-go/1.5/pkg/api"
	v1 "k8s.io/client-go/1.5/pkg/api/v1"
	apiext "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"
	rest "k8s.io/client-go/1.5/rest"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
)

type extendedClient struct {
	discovery.DiscoveryInterface
	v1core.CoreInterface
	v1beta1extensions.ExtensionsInterface
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

// --- add-ons

// Kubernetes has a mechanism of "Add-ons", whereby manifest files
// left in a particular directory on the Kubernetes master will be
// applied. We can recognise these, because they:
//  1. Must be in the namespace `kube-system`; and,
//  2. Must have one of the labels below set, else the addon manager will ignore them.
//
// We want to ignore add-ons, since they are managed by the add-on
// manager, and attempts to control them via other means will fail.

type namespacedLabeled interface {
	GetNamespace() string
	GetLabels() map[string]string
}

func isAddon(obj namespacedLabeled) bool {
	if obj.GetNamespace() != "kube-system" {
		return false
	}
	labels := obj.GetLabels()
	if labels["kubernetes.io/cluster-service"] == "true" ||
		labels["addonmanager.kubernetes.io/mode"] == "EnsureExists" ||
		labels["addonmanager.kubernetes.io/mode"] == "Reconcile" {
		return true
	}
	return false
}

// --- /add ons

// Cluster is a handle to a Kubernetes API server.
// (Typically, this code is deployed into the same cluster.)
type Cluster struct {
	config  *rest.Config
	client  extendedClient
	kubectl string
	actionc chan func()
	version string // string response for the version command.
	logger  log.Logger
}

// NewCluster returns a usable cluster. Host should be of the form
// "http://hostname:8080".
func NewCluster(config *rest.Config, kubectl, version string, logger log.Logger) (*Cluster, error) {
	client, err := k8sclient.NewForConfig(config)
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
		client:  extendedClient{client.Discovery(), client.Core(), client.Extensions()},
		kubectl: kubectl,
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
				continue
			}
			if isAddon(service) {
				continue
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
			if isAddon(&service) {
				continue
			}
			if !ignore.Contains(flux.MakeServiceID(ns, service.Name)) {
				res = append(res, c.makeService(ns, &service, controllers))
			}
		}
	}
	return res, nil
}

func (c *Cluster) makeService(ns string, service *v1.Service, controllers []podController) platform.Service {
	id := flux.MakeServiceID(ns, service.Name)
	// FIXME go look in the k8s API
	var status = ""
	return platform.Service{
		ID:         id,
		IP:         service.Spec.ClusterIP,
		Metadata:   metadataForService(service),
		Containers: containersOrExcuse(service, controllers),
		Status:     status,
	}
}

func metadataForService(s *v1.Service) map[string]string {
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
		if !isAddon(&deploylist.Items[i]) {
			res = append(res, podController{Deployment: &deploylist.Items[i]})
		}
	}

	rclist, err := c.client.ReplicationControllers(namespace).List(api.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "collecting replication controllers")
	}
	for i := range rclist.Items {
		if !isAddon(&rclist.Items[i]) {
			res = append(res, podController{ReplicationController: &rclist.Items[i]})
		}
	}

	return res, nil
}

// Find the pod controller (deployment or replication controller) that matches the service
func matchController(service *v1.Service, controllers []podController) (podController, error) {
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

func containersOrExcuse(service *v1.Service, controllers []podController) platform.ContainersOrExcuse {
	pc, err := matchController(service, controllers)
	if err != nil {
		return platform.ContainersOrExcuse{Excuse: err.Error()}
	}
	return platform.ContainersOrExcuse{Containers: pc.templateContainers()}
}

// Either a replication controller, a deployment, or neither (both nils).
type podController struct {
	ReplicationController *v1.ReplicationController
	Deployment            *apiext.Deployment
}

func (p podController) templateContainers() (res []platform.Container) {
	var apiContainers []v1.Container
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

// Sync performs the given actions on resources. Operations are
// asynchronous, but serialised.
func (c *Cluster) Sync(spec platform.SyncDef) error {
	errc := make(chan error)
	logger := log.NewContext(c.logger).With("method", "Sync")
	c.actionc <- func() {
		errs := platform.SyncError{}
		for id, action := range spec.Actions {
			if len(action.Delete) > 0 {
				if err := c.doCommand(logger, action.Delete, "delete", "-f", "-"); err != nil {
					errs[id] = err
					continue
				}
			}
			if len(action.Create) > 0 {
				if err := c.doCommand(logger, action.Create, "create", "-f", "-"); err != nil {
					errs[id] = err
					continue
				}
			}
			if len(action.Apply) > 0 {
				// special case for rolling upgrades
				obj, err := definitionObj(action.Apply)
				if err != nil {
					errs[id] = err
					continue
				}
				switch obj.Kind {
				case "ReplicationController":
					err = c.startRollingUpgrade(logger, obj)
				default:
					err = c.doCommand(logger, action.Apply, "apply", "-f", "-")
				}
				if err != nil {
					errs[id] = err
					continue
				}
			}
		}
		if len(errs) > 0 {
			errc <- errs
		}
		errc <- nil
	}
	return <-errc
}

// Apply applies a new set of ServiceDefinition. If all definitions succeed,
// Apply returns a nil error. If any definitions fail, Apply returns an error
// of type ApplyError, which can be inspected for more detailed information.
// Applies are serialized.
func (c *Cluster) Apply(defs []platform.ServiceDefinition) error {
	sync := platform.SyncDef{Actions: map[platform.ResourceID]platform.SyncAction{}}
	for _, def := range defs {
		id := platform.ResourceID(def.ServiceID.String())
		sync.Actions[id] = platform.SyncAction{Apply: def.NewDefinition}
	}
	err := c.Sync(sync)
	if err == nil {
		return nil
	}

	switch err := err.(type) {
	case platform.SyncError:
		applyErr := platform.ApplyError{}
		for rid, syncErr := range err {
			applyErr[flux.ServiceID(rid.String())] = syncErr
		}
		return applyErr
	default:
		return err
	}
}

func (c *Cluster) Ping() error {
	_, err := c.client.ServerVersion()
	return err
}

func (c *Cluster) Version() (string, error) {
	return c.version, nil
}

func (c *Cluster) Export() ([]byte, error) {
	var config bytes.Buffer
	list, err := c.client.Namespaces().List(api.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}
	for _, ns := range list.Items {
		err := appendYAML(&config, "v1", "Namespace", ns)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling namespace to YAML")
		}

		deployments, err := c.client.Deployments(ns.Name).List(api.ListOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "getting deployments")
		}
		for _, deployment := range deployments.Items {
			if isAddon(&deployment) {
				continue
			}
			err := appendYAML(&config, "extensions/v1beta1", "Deployment", deployment)
			if err != nil {
				return nil, errors.Wrap(err, "marshalling deployment to YAML")
			}
		}

		rcs, err := c.client.ReplicationControllers(ns.Name).List(api.ListOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "getting replication controllers")
		}
		for _, rc := range rcs.Items {
			if isAddon(&rc) {
				continue
			}
			err := appendYAML(&config, "v1", "ReplicationController", rc)
			if err != nil {
				return nil, errors.Wrap(err, "marshalling replication controller to YAML")
			}
		}

		services, err := c.client.Services(ns.Name).List(api.ListOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "getting services")
		}
		for _, service := range services.Items {
			if isAddon(&service) {
				continue
			}
			err := appendYAML(&config, "v1", "Service", service)
			if err != nil {
				return nil, errors.Wrap(err, "marshalling service to YAML")
			}
		}
	}
	return config.Bytes(), nil
}

// kind & apiVersion must be passed separately as the object's TypeMeta is not populated
func appendYAML(buffer *bytes.Buffer, apiVersion, kind string, object interface{}) error {
	yamlBytes, err := k8syaml.Marshal(object)
	if err != nil {
		return err
	}
	buffer.WriteString("---\n")
	buffer.WriteString("apiVersion: ")
	buffer.WriteString(apiVersion)
	buffer.WriteString("\nkind: ")
	buffer.WriteString(kind)
	buffer.WriteString("\n")
	buffer.Write(yamlBytes)
	return nil
}

// --- end platform API

// A convenience for getting an minimal object from some bytes.
func definitionObj(bytes []byte) (*apiObject, error) {
	obj := apiObject{bytes: bytes}
	return &obj, yaml.Unmarshal(bytes, &obj)
}
