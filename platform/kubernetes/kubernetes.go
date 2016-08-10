// Package kubernetes provides abstractions for the Kubernetes platform. At the
// moment, Kubernetes is the only supported platform, so we are directly
// returning Kubernetes objects. As we add more platforms, we will create
// abstractions and common data types in package platform.
package kubernetes

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"gopkg.in/yaml.v2"
	"k8s.io/kubernetes/pkg/api"
	k8serrors "k8s.io/kubernetes/pkg/api/errors"
	apiext "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/restclient"
	k8sclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/weaveworks/fluxy/platform"
)

// These errors are returned by cluster methods.
var (
	ErrEmptySelector        = errors.New("empty selector")
	ErrWrongResourceKind    = errors.New("new definition does not match existing resource")
	ErrNoMatchingService    = errors.New("no matching service")
	ErrServiceHasNoSelector = errors.New("service has no selector")
	ErrNoMatching           = errors.New("no matching replication controllers or deployments")
	ErrMultipleMatching     = errors.New("multiple matching replication controllers or deployments")
	ErrNoMatchingImages     = errors.New("no matching images")
)

type extendedClient struct {
	*k8sclient.Client
	*k8sclient.ExtensionsClient
}

// Cluster is a handle to a Kubernetes API server.
// (Typically, this code is deployed into the same cluster.)
type Cluster struct {
	config  *restclient.Config
	client  extendedClient
	kubectl string
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
		logger:  logger,
	}, nil
}

// Service returns the platform.Service representation of the named service.
func (c *Cluster) Service(namespace, service string) (platform.Service, error) {
	apiService, err := c.service(namespace, service)
	if err != nil {
		if statusErr, ok := err.(*k8serrors.StatusError); ok && statusErr.ErrStatus.Code == http.StatusNotFound { // le sigh
			return platform.Service{}, ErrNoMatchingService
		}
		return platform.Service{}, err
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
		return nil, err
	}
	return c.makePlatformServices(apiServices), nil
}

func (c *Cluster) service(namespace, service string) (res api.Service, err error) {
	defer func() {
		c.logger.Log("method", "service", "namespace", namespace, "service", service, "err", err)
	}()
	apiService, err := c.client.Services(namespace).Get(service)
	if err != nil {
		return api.Service{}, err
	}
	return *apiService, nil
}

func (c *Cluster) services(namespace string) (res []api.Service, err error) {
	defer func() {
		c.logger.Log("method", "services", "namespace", namespace, "count", len(res), "err", err)
	}()
	list, err := c.client.Services(namespace).List(api.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

type apiObject struct {
	Version string `yaml:"apiVersion"`
	Kind    string `yaml:"kind"`
}

func definitionObj(bytes []byte) (*apiObject, error) {
	var obj apiObject
	return &obj, yaml.Unmarshal(bytes, &obj)
}

// Release performs a update of the service, from whatever it is
// currently, to what is described by the new resource, which can be a
// replication controller or deployment.
//
// Release assumes there is a one-to-one mapping between services and
// replication controllers or deployments; this can be
// improved. Release blocks until the rolling update is complete; this
// can be improved. Release invokes `kubectl rolling-update` or
// `kubectl apply` in a seperate process, and assumes kubectl is in
// the PATH; this can be improved.
func (c *Cluster) Release(namespace, serviceName string, newDefinition []byte, updatePeriod time.Duration) error {
	logger := log.NewContext(c.logger).With("method", "Release", "namespace", namespace, "service", serviceName)
	logger.Log()

	obj, err := definitionObj(newDefinition)
	if err != nil {
		return err
	}

	pc, err := c.podControllerFor(namespace, serviceName)
	if err != nil {
		return err
	}

	var release releaseProc
	ns := namespacedService{namespace, serviceName}
	if pc.Deployment != nil {
		if obj.Kind != "Deployment" {
			return ErrWrongResourceKind
		}
		release = releaseDeployment{ns, c, pc.Deployment}
	} else if pc.ReplicationController != nil {
		if obj.Kind != "ReplicationController" {
			return ErrWrongResourceKind
		}
		release = releaseReplicationController{ns, c, pc.ReplicationController, updatePeriod}
	} else {
		return ErrNoMatching
	}
	return release.do(newDefinition, logger)
}

type namespacedService struct {
	namespace string
	service   string
}

type releaseProc interface {
	do(newDefinition []byte, logger log.Logger) error
}

type releaseReplicationController struct {
	namespacedService
	cluster      *Cluster
	rc           *api.ReplicationController
	updatePeriod time.Duration
}

func (c releaseReplicationController) do(newDefinition []byte, logger log.Logger) error {
	var args []string
	if c.cluster.config.Host != "" {
		args = append(args, fmt.Sprintf("--server=%s", c.cluster.config.Host))
	}
	if c.cluster.config.Username != "" {
		args = append(args, fmt.Sprintf("--username=%s", c.cluster.config.Username))
	}
	if c.cluster.config.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", c.cluster.config.Password))
	}
	if c.cluster.config.TLSClientConfig.CertFile != "" {
		args = append(args, fmt.Sprintf("--client-certificate=%s", c.cluster.config.TLSClientConfig.CertFile))
	}
	if c.cluster.config.TLSClientConfig.CAFile != "" {
		args = append(args, fmt.Sprintf("--certificate-authority=%s", c.cluster.config.TLSClientConfig.CAFile))
	}
	if c.cluster.config.TLSClientConfig.KeyFile != "" {
		args = append(args, fmt.Sprintf("--client-key=%s", c.cluster.config.TLSClientConfig.KeyFile))
	}
	if c.cluster.config.BearerToken != "" {
		args = append(args, fmt.Sprintf("--token=%s", c.cluster.config.BearerToken))
	}
	args = append(args, []string{
		"rolling-update",
		c.rc.Name,
		fmt.Sprintf("--update-period=%s", c.updatePeriod),
		"-f", "-", // take definition from stdin
	}...)

	cmd := exec.Command(c.cluster.kubectl, args...)
	cmd.Stdin = bytes.NewReader(newDefinition)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logger.Log("cmd", strings.Join(cmd.Args, " "))

	begin := time.Now()
	err := cmd.Run()
	result := "success"
	if err != nil {
		result = err.Error()
	}
	logger.Log("result", result, "took", time.Since(begin).String())

	return err
}

type releaseDeployment struct {
	namespacedService
	cluster *Cluster
	d       *apiext.Deployment
}

func (c releaseDeployment) do(newDefinition []byte, logger log.Logger) error {
	var args []string
	if c.cluster.config.Host != "" {
		args = append(args, fmt.Sprintf("--server=%s", c.cluster.config.Host))
	}
	if c.cluster.config.Username != "" {
		args = append(args, fmt.Sprintf("--username=%s", c.cluster.config.Username))
	}
	if c.cluster.config.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", c.cluster.config.Password))
	}
	if c.cluster.config.TLSClientConfig.CertFile != "" {
		args = append(args, fmt.Sprintf("--client-certificate=%s", c.cluster.config.TLSClientConfig.CertFile))
	}
	if c.cluster.config.TLSClientConfig.CAFile != "" {
		args = append(args, fmt.Sprintf("--certificate-authority=%s", c.cluster.config.TLSClientConfig.CAFile))
	}
	if c.cluster.config.TLSClientConfig.KeyFile != "" {
		args = append(args, fmt.Sprintf("--client-key=%s", c.cluster.config.TLSClientConfig.KeyFile))
	}
	if c.cluster.config.BearerToken != "" {
		args = append(args, fmt.Sprintf("--token=%s", c.cluster.config.BearerToken))
	}
	args = append(args, []string{
		"apply",
		"-f", "-", // take definition from stdin
	}...)

	cmd := exec.Command(c.cluster.kubectl, args...)
	cmd.Stdin = bytes.NewReader(newDefinition)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logger.Log("cmd", strings.Join(cmd.Args, " "))

	begin := time.Now()
	err := cmd.Run()
	result := "success"
	if err != nil {
		result = err.Error()
	}
	logger.Log("result", result, "took", time.Since(begin).String())

	return err
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

func (p podController) templateContainers() []api.Container {
	if p.Deployment != nil {
		return p.Deployment.Spec.Template.Spec.Containers
	} else if p.ReplicationController != nil {
		return p.ReplicationController.Spec.Template.Spec.Containers
	}
	return nil
}

func (c *Cluster) podControllerFor(namespace, serviceName string) (res podController, err error) {
	logger := log.NewContext(c.logger).With("method", "podControllerFor", "namespace", namespace, "serviceName", serviceName)
	defer func() {
		if err != nil {
			logger.Log("err", err.Error())
		} else {
			logger.Log("rc", res.name())
		}
	}()

	res = podController{}

	// First, get the service spec selector, which determines the pods that the
	// service will load balance over.
	services, err := c.services(namespace)
	if err != nil {
		return res, err
	}
	service, ok := func() (api.Service, bool) {
		for _, service := range services {
			if service.Name == serviceName { // assume names are unique
				return service, true
			}
		}
		return api.Service{}, false
	}()
	if !ok {
		return res, ErrNoMatchingService
	}

	selector := service.Spec.Selector
	if len(selector) <= 0 {
		return res, ErrServiceHasNoSelector
	}

	// Now, try to find a deployment or replication controller that
	// matches the selector given in the service. The simplifying
	// assumption for the time being is that there's just one of these
	// -- we return an error otherwise.

	// Find a replication controller which produces pods that match that
	// selector. We have to match all of the criteria in the selector, but we
	// don't need a perfect match of all of the replication controller's pod
	// properties.
	list, err := c.client.ReplicationControllers(namespace).List(api.ListOptions{})
	if err != nil {
		return res, err
	}
	var rcs []api.ReplicationController
	for _, rc := range list.Items {
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
		return res, ErrMultipleMatching
	}

	deplist, err := c.client.Deployments(namespace).List(api.ListOptions{})
	if err != nil {
		return res, err
	}

	deps := []apiext.Deployment{}
	for _, d := range deplist.Items {
		match := func() bool {
			// For each key=value pair in the service spec, check if the RC
			// annotates its pods in the same way. If any rule fails, the RC is
			// not a match. If all rules pass, the RC is a match.
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
		return res, ErrMultipleMatching
	}

	if res.ReplicationController != nil && res.Deployment != nil {
		return res, ErrMultipleMatching
	}
	if res.ReplicationController == nil && res.Deployment == nil {
		return res, ErrNoMatching
	}
	return res, nil
}

// ContainersFor returns a list of container names with the image
// specified to run in that container, for a particular service. This
// is useful to see which images a particular service is presently
// running, to judge whether a release is needed.
func (c *Cluster) ContainersFor(namespace, serviceName string) (res []platform.Container, err error) {
	logger := log.NewContext(c.logger).With("method", "ImagesFor", "namespace", namespace, "serviceName", serviceName)
	defer func() {
		if err != nil {
			logger.Log("err", err.Error())
		} else {
			logger.Log("containers", fmt.Sprintf("%+v", res))
		}
	}()

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
		return nil, ErrNoMatchingImages
	}
	return containers, nil
}

func (c *Cluster) imagesFor(namespace, imageName string) ([]string, error) {
	containers, err := c.ContainersFor(namespace, imageName)
	if err != nil {
		return nil, err
	}
	images := make([]string, len(containers))
	for i := 0; i < len(containers); i++ {
		images[i] = containers[i].Image
	}
	return images, nil
}

func (c *Cluster) makePlatformServices(apiServices []api.Service) []platform.Service {
	platformServices := make([]platform.Service, len(apiServices))
	for i, s := range apiServices {
		platformServices[i] = c.makePlatformService(s)
	}
	return platformServices
}

func (c *Cluster) makePlatformService(s api.Service) platform.Service {
	// To get the image, we need to walk from service to RC to spec.
	// That path encodes a lot of opinions about deployment strategy.
	// Which we're OK with, for the time being.
	var image string
	switch images, err := c.imagesFor(s.Namespace, s.Name); err {
	case nil:
		image = strings.Join(images, ", ") // >1 image would break some light assumptions, but it's OK
	case ErrServiceHasNoSelector:
		image = "(no selector, no RC)"
	case ErrNoMatching:
		image = "(no RC or Deployment)"
	case ErrMultipleMatching:
		image = "(multiple RCs/Deployments)" // e.g. during a release
	case ErrNoMatchingImages:
		image = "(none)"
	}

	ports := make([]platform.Port, len(s.Spec.Ports))
	for i, port := range s.Spec.Ports {
		ports[i] = platform.Port{
			External: fmt.Sprint(port.Port),
			Internal: port.TargetPort.String(),
			Protocol: string(port.Protocol),
		}
	}

	metadata := map[string]string{
		"created_at":       s.CreationTimestamp.String(),
		"resource_version": s.ResourceVersion,
		"uid":              string(s.UID),
		"type":             string(s.Spec.Type),
	}

	return platform.Service{
		Name:     s.Name,
		Image:    image,
		IP:       s.Spec.ClusterIP,
		Ports:    ports,
		Metadata: metadata,
	}
}
