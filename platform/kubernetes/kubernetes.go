// Package kubernetes provides abstractions for the Kubernetes platform. At the
// moment, Kubernetes is the only supported platform, so we are directly
// returning Kubernetes objects. As we add more platforms, we will create
// abstractions and common data types in package platform.
package kubernetes

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	k8sclient "k8s.io/kubernetes/pkg/client/unversioned"
)

// These errors are returned by cluster methods.
var (
	ErrEmptySelector                          = errors.New("empty selector")
	ErrNoMatchingService                      = errors.New("no matching service")
	ErrNoMatchingReplicationController        = errors.New("no matching replication controllers")
	ErrMultipleMatchingReplicationControllers = errors.New("multiple matching replication controllers")
)

// Cluster is a handle to a Kubernetes API server.
// (Typically, this code is deployed into the same cluster.)
type Cluster struct {
	config  *restclient.Config
	client  *k8sclient.Client
	kubectl string
	logger  log.Logger
}

// NewCluster returns a usable cluster. Host should be of the form
// "http://hostname:8080".
func NewCluster(config *restclient.Config, logger log.Logger) (*Cluster, error) {
	client, err := k8sclient.New(config)
	if err != nil {
		return nil, err
	}

	kubectl, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, err
	}
	logger.Log("kubectl", kubectl)

	return &Cluster{
		config:  config,
		client:  client,
		kubectl: kubectl,
		logger:  logger,
	}, nil
}

// Services returns the set of services currently active on the platform in the
// given namespace. Maybe it makes sense to move the namespace to the
// constructor? Depends on how it will be used. For now it is here.
//
// The user is expected to list services, and then choose the one that will
// receive a release. Releases operate on replication controllers, not services.
// For now, we make a simplifying assumption that there is a one-to-one mapping
// between services and replication controllers.
func (c *Cluster) Services(namespace string) ([]api.Service, error) {
	list, err := c.client.Services(namespace).List(api.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// Release performs a rolling update of the service, from whatever it is
// currently, to what is described by the new replication controller. Release
// assumes there is a one-to-one mapping between services and replication
// controllers; this can be improved. Release blocks until the rolling update is
// complete; this can be improved. Release invokes `kubectl rolling-update` in a
// seperate process, and assumes kubectl is in the PATH; this can be improved.
func (c *Cluster) Release(namespace, serviceName string, newReplicationControllerDefinition []byte, updatePeriod time.Duration) error {
	logger := log.NewContext(c.logger).With("method", "Release", "namespace", namespace, "service", serviceName)
	logger.Log()

	rc, err := c.replicationControllerFor(namespace, serviceName)
	if err != nil {
		return err
	}
	logger.Log("RC", rc.Name)

	var args []string
	if c.config.Host != "" {
		args = append(args, fmt.Sprintf("--server=%s", c.config.Host))
	}
	if c.config.Username != "" {
		args = append(args, fmt.Sprintf("--username=%s", c.config.Username))
	}
	if c.config.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", c.config.Password))
	}
	if c.config.TLSClientConfig.CertFile != "" {
		args = append(args, fmt.Sprintf("--client-certificate=%s", c.config.TLSClientConfig.CertFile))
	}
	if c.config.TLSClientConfig.KeyFile != "" {
		args = append(args, fmt.Sprintf("--client-key=%s", c.config.KeyFile))
	}
	args = append(args, []string{
		"--validate=false", // for some reason, this is required with our defs
		"rolling-update",
		rc.Name,
		fmt.Sprintf("--update-period=%s", updatePeriod),
		"-f", "-", // take definition from stdin
	}...)

	cmd := exec.Command(c.kubectl, args...)
	cmd.Stdin = bytes.NewReader(newReplicationControllerDefinition)
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = ioutil.Discard
	logger.Log("cmd", strings.Join(cmd.Args, " "))

	begin := time.Now()
	err = cmd.Run()
	result := "success"
	if err != nil {
		result = err.Error()
	}
	logger.Log("result", result, "took", time.Since(begin).String())

	return err
}

func (c *Cluster) replicationControllerFor(namespace, serviceName string) (api.ReplicationController, error) {
	logger := log.NewContext(c.logger).With("method", "replicationControllerFor", "namespace", namespace, "serviceName", serviceName)
	logger.Log()

	// First, get the service spec selector, which determines the pods that the
	// service will load balance over.
	services, err := c.Services(namespace)
	if err != nil {
		return api.ReplicationController{}, err
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
		return api.ReplicationController{}, ErrNoMatchingService
	}
	selector := service.Spec.Selector

	// Now, find a replication controller which produces pods that match that
	// selector. We have to match all of the criteria in the selector, but we
	// don't need a perfect match of all of the replication controller's pod
	// properties.
	list, err := c.client.ReplicationControllers(namespace).List(api.ListOptions{})
	if err != nil {
		return api.ReplicationController{}, err
	}
	var matches []api.ReplicationController
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
			matches = append(matches, rc)
		}
	}

	// Our naïve, simplifying assumption: every service is satisfied by
	// precisely 1 replication controller.
	switch len(matches) {
	case 0:
		return api.ReplicationController{}, ErrNoMatchingReplicationController
	case 1:
		return matches[0], nil
	default:
		return api.ReplicationController{}, ErrMultipleMatchingReplicationControllers
	}
}
