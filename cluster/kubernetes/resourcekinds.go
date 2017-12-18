package kubernetes

import (
	"fmt"

	apiapps "k8s.io/api/apps/v1beta1"
	apibatch "k8s.io/api/batch/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	apiext "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
)

/////////////////////////////////////////////////////////////////////////////
// Kind registry

type resourceKind interface {
	getPodController(c *Cluster, namespace, name string) (podController, error)
	getPodControllers(c *Cluster, namespace string) ([]podController, error)
}

var (
	resourceKinds = make(map[string]resourceKind)
)

func init() {
	resourceKinds["cronjob"] = &cronJobKind{}
	resourceKinds["daemonset"] = &daemonSetKind{}
	resourceKinds["deployment"] = &deploymentKind{}
	resourceKinds["statefulset"] = &statefulSetKind{}
}

type podController struct {
	apiVersion  string
	kind        string
	name        string
	status      string
	podTemplate apiv1.PodTemplateSpec
	apiObject   interface{}
}

func (pc podController) toClusterController(resourceID flux.ResourceID) cluster.Controller {
	var clusterContainers []cluster.Container
	for _, container := range pc.podTemplate.Spec.Containers {
		clusterContainers = append(clusterContainers, cluster.Container{Name: container.Name, Image: container.Image})
	}

	return cluster.Controller{
		ID:         resourceID,
		Status:     pc.status,
		Containers: cluster.ContainersOrExcuse{Containers: clusterContainers},
	}
}

func (pc podController) GetNamespace() string {
	objectMeta := pc.apiObject.(namespacedLabeled)
	return objectMeta.GetNamespace()
}

func (pc podController) GetLabels() map[string]string {
	objectMeta := pc.apiObject.(namespacedLabeled)
	return objectMeta.GetLabels()
}

/////////////////////////////////////////////////////////////////////////////
// extensions/v1beta1 Deployment

type deploymentKind struct{}

func (dk *deploymentKind) getPodController(c *Cluster, namespace, name string) (podController, error) {
	deployment, err := c.client.Deployments(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return podController{}, err
	}

	return makeDeploymentPodController(deployment), nil
}

func (dk *deploymentKind) getPodControllers(c *Cluster, namespace string) ([]podController, error) {
	deployments, err := c.client.Deployments(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var podControllers []podController
	for i := range deployments.Items {
		podControllers = append(podControllers, makeDeploymentPodController(&deployments.Items[i]))
	}

	return podControllers, nil
}

func makeDeploymentPodController(deployment *apiext.Deployment) podController {
	var status string
	objectMeta, deploymentStatus := deployment.ObjectMeta, deployment.Status

	if deploymentStatus.ObservedGeneration >= objectMeta.Generation {
		// the definition has been updated; now let's see about the replicas
		updated, wanted := deploymentStatus.UpdatedReplicas, *deployment.Spec.Replicas
		if updated == wanted {
			status = StatusReady
		} else {
			status = fmt.Sprintf("%d out of %d updated", updated, wanted)
		}
	} else {
		status = StatusUpdating
	}

	return podController{
		apiVersion:  "extensions/v1beta1",
		kind:        "Deployment",
		name:        deployment.ObjectMeta.Name,
		status:      status,
		podTemplate: deployment.Spec.Template,
		apiObject:   deployment}
}

/////////////////////////////////////////////////////////////////////////////
// extensions/v1beta daemonset

type daemonSetKind struct{}

func (dk *daemonSetKind) getPodController(c *Cluster, namespace, name string) (podController, error) {
	daemonSet, err := c.client.DaemonSets(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return podController{}, err
	}

	return makeDaemonSetPodController(daemonSet), nil
}

func (dk *daemonSetKind) getPodControllers(c *Cluster, namespace string) ([]podController, error) {
	daemonSets, err := c.client.DaemonSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var podControllers []podController
	for i, _ := range daemonSets.Items {
		podControllers = append(podControllers, makeDaemonSetPodController(&daemonSets.Items[i]))
	}

	return podControllers, nil
}

func makeDaemonSetPodController(daemonSet *apiext.DaemonSet) podController {
	var status string
	objectMeta, daemonSetStatus := daemonSet.ObjectMeta, daemonSet.Status
	if daemonSetStatus.ObservedGeneration >= objectMeta.Generation {
		// the definition has been updated; now let's see about the replicas
		updated, wanted := daemonSetStatus.UpdatedNumberScheduled, daemonSetStatus.DesiredNumberScheduled
		if updated == wanted {
			status = StatusReady
		} else {
			status = fmt.Sprintf("%d out of %d updated", updated, wanted)
		}
	} else {
		status = StatusUpdating
	}

	return podController{
		apiVersion:  "extensions/v1beta1",
		kind:        "DaemonSet",
		name:        daemonSet.ObjectMeta.Name,
		status:      status,
		podTemplate: daemonSet.Spec.Template,
		apiObject:   daemonSet}
}

/////////////////////////////////////////////////////////////////////////////
// apps/v1beta1 StatefulSet

type statefulSetKind struct{}

func (dk *statefulSetKind) getPodController(c *Cluster, namespace, name string) (podController, error) {
	statefulSet, err := c.client.StatefulSets(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return podController{}, err
	}

	return makeStatefulSetPodController(statefulSet), nil
}

func (dk *statefulSetKind) getPodControllers(c *Cluster, namespace string) ([]podController, error) {
	statefulSets, err := c.client.StatefulSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var podControllers []podController
	for i, _ := range statefulSets.Items {
		podControllers = append(podControllers, makeStatefulSetPodController(&statefulSets.Items[i]))
	}

	return podControllers, nil
}

func makeStatefulSetPodController(statefulSet *apiapps.StatefulSet) podController {
	var status string
	objectMeta, statefulSetStatus := statefulSet.ObjectMeta, statefulSet.Status
	if *statefulSetStatus.ObservedGeneration >= objectMeta.Generation {
		// the definition has been updated; now let's see about the replicas
		updated, wanted := statefulSetStatus.UpdatedReplicas, *statefulSet.Spec.Replicas
		if updated == wanted {
			status = StatusReady
		} else {
			status = fmt.Sprintf("%d out of %d updated", updated, wanted)
		}
	} else {
		status = StatusUpdating
	}

	return podController{
		apiVersion:  "apps/v1beta1",
		kind:        "StatefulSet",
		name:        statefulSet.ObjectMeta.Name,
		status:      status,
		podTemplate: statefulSet.Spec.Template,
		apiObject:   statefulSet}
}

/////////////////////////////////////////////////////////////////////////////
// batch/v1beta1 CronJob

type cronJobKind struct{}

func (dk *cronJobKind) getPodController(c *Cluster, namespace, name string) (podController, error) {
	cronJob, err := c.client.CronJobs(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return podController{}, err
	}

	return makeCronJobPodController(cronJob), nil
}

func (dk *cronJobKind) getPodControllers(c *Cluster, namespace string) ([]podController, error) {
	cronJobs, err := c.client.CronJobs(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var podControllers []podController
	for i, _ := range cronJobs.Items {
		podControllers = append(podControllers, makeCronJobPodController(&cronJobs.Items[i]))
	}

	return podControllers, nil
}

func makeCronJobPodController(cronJob *apibatch.CronJob) podController {
	return podController{
		apiVersion:  "batch/v1beta1",
		kind:        "CronJob",
		name:        cronJob.ObjectMeta.Name,
		status:      StatusReady,
		podTemplate: cronJob.Spec.JobTemplate.Spec.Template,
		apiObject:   cronJob}
}

/////////////////////////////////////////////////////////////////////////////
//
