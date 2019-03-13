package kubernetes

import (
	"strings"

	apiapps "k8s.io/api/apps/v1"
	apibatch "k8s.io/api/batch/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/image"
	fhr_v1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	fhr_v1alpha2 "github.com/weaveworks/flux/integrations/apis/helm.integrations.flux.weave.works/v1alpha2"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

// AntecedentAnnotation is an annotation on a resource indicating that
// the cause of that resource (indirectly, via a Helm release) is a
// FluxHelmRelease. We use this rather than the `OwnerReference` type
// built into Kubernetes so that there are no garbage-collection
// implications. The value is expected to be a serialised
// `flux.ResourceID`.
const AntecedentAnnotation = "flux.weave.works/antecedent"

/////////////////////////////////////////////////////////////////////////////
// Kind registry

type resourceKind interface {
	getWorkload(c *Cluster, namespace, name string) (workload, error)
	getWorkloads(c *Cluster, namespace string) ([]workload, error)
}

var (
	resourceKinds = make(map[string]resourceKind)
)

func init() {
	resourceKinds["cronjob"] = &cronJobKind{}
	resourceKinds["daemonset"] = &daemonSetKind{}
	resourceKinds["deployment"] = &deploymentKind{}
	resourceKinds["statefulset"] = &statefulSetKind{}
	resourceKinds["helmrelease"] = &helmReleaseKind{}
	resourceKinds["fluxhelmrelease"] = &fluxHelmReleaseKind{}
}

type workload struct {
	k8sObject
	apiVersion  string
	kind        string
	name        string
	status      string
	rollout     cluster.RolloutStatus
	syncError   error
	podTemplate apiv1.PodTemplateSpec
}

func (w workload) toClusterWorkload(resourceID flux.ResourceID) cluster.Workload {
	var clusterContainers []resource.Container
	var excuse string
	for _, container := range w.podTemplate.Spec.Containers {
		ref, err := image.ParseRef(container.Image)
		if err != nil {
			clusterContainers = nil
			excuse = err.Error()
			break
		}
		clusterContainers = append(clusterContainers, resource.Container{Name: container.Name, Image: ref})
	}
	for _, container := range w.podTemplate.Spec.InitContainers {
		ref, err := image.ParseRef(container.Image)
		if err != nil {
			clusterContainers = nil
			excuse = err.Error()
			break
		}
		clusterContainers = append(clusterContainers, resource.Container{Name: container.Name, Image: ref})
	}

	var antecedent flux.ResourceID
	if ante, ok := w.GetAnnotations()[AntecedentAnnotation]; ok {
		id, err := flux.ParseResourceID(ante)
		if err == nil {
			antecedent = id
		}
	}

	var policies policy.Set
	for k, v := range w.GetAnnotations() {
		if strings.HasPrefix(k, kresource.PolicyPrefix) {
			p := strings.TrimPrefix(k, kresource.PolicyPrefix)
			if v == "true" {
				policies = policies.Add(policy.Policy(p))
			} else {
				policies = policies.Set(policy.Policy(p), v)
			}
		}
	}

	return cluster.Workload{
		ID:         resourceID,
		Status:     w.status,
		Rollout:    w.rollout,
		SyncError:  w.syncError,
		Antecedent: antecedent,
		Labels:     w.GetLabels(),
		Policies:   policies,
		Containers: cluster.ContainersOrExcuse{Containers: clusterContainers, Excuse: excuse},
	}
}

/////////////////////////////////////////////////////////////////////////////
// extensions/v1beta1 Deployment

type deploymentKind struct{}

func (dk *deploymentKind) getWorkload(c *Cluster, namespace, name string) (workload, error) {
	deployment, err := c.client.AppsV1().Deployments(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}

	return makeDeploymentWorkload(deployment), nil
}

func (dk *deploymentKind) getWorkloads(c *Cluster, namespace string) ([]workload, error) {
	deployments, err := c.client.AppsV1().Deployments(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var workloads []workload
	for i := range deployments.Items {
		workloads = append(workloads, makeDeploymentWorkload(&deployments.Items[i]))
	}

	return workloads, nil
}

// Deployment may get stuck trying to deploy its newest ReplicaSet without ever completing.
// One way to detect this condition is to specify a deadline parameter in Deployment spec:
// .spec.progressDeadlineSeconds
// See https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#failed-deployment
func deploymentErrors(d *apiapps.Deployment) []string {
	var errs []string
	for _, cond := range d.Status.Conditions {
		if (cond.Type == apiapps.DeploymentProgressing && cond.Status == apiv1.ConditionFalse) ||
			(cond.Type == apiapps.DeploymentReplicaFailure && cond.Status == apiv1.ConditionTrue) {
			errs = append(errs, cond.Message)
		}
	}
	return errs
}

func makeDeploymentWorkload(deployment *apiapps.Deployment) workload {
	var status string
	objectMeta, deploymentStatus := deployment.ObjectMeta, deployment.Status

	status = cluster.StatusStarted
	rollout := cluster.RolloutStatus{
		Desired:   *deployment.Spec.Replicas,
		Updated:   deploymentStatus.UpdatedReplicas,
		Ready:     deploymentStatus.ReadyReplicas,
		Available: deploymentStatus.AvailableReplicas,
		Outdated:  deploymentStatus.Replicas - deploymentStatus.UpdatedReplicas,
		Messages:  deploymentErrors(deployment),
	}

	if deploymentStatus.ObservedGeneration >= objectMeta.Generation {
		// the definition has been updated; now let's see about the replicas
		status = cluster.StatusUpdating
		if rollout.Updated == rollout.Desired && rollout.Available == rollout.Desired && rollout.Outdated == 0 {
			status = cluster.StatusReady
		}
		if len(rollout.Messages) != 0 {
			status = cluster.StatusError
		}
	}

	return workload{
		apiVersion:  "apps/v1",
		kind:        "Deployment",
		name:        deployment.ObjectMeta.Name,
		status:      status,
		rollout:     rollout,
		podTemplate: deployment.Spec.Template,
		k8sObject:   deployment}
}

/////////////////////////////////////////////////////////////////////////////
// extensions/v1beta daemonset

type daemonSetKind struct{}

func (dk *daemonSetKind) getWorkload(c *Cluster, namespace, name string) (workload, error) {
	daemonSet, err := c.client.AppsV1().DaemonSets(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}

	return makeDaemonSetWorkload(daemonSet), nil
}

func (dk *daemonSetKind) getWorkloads(c *Cluster, namespace string) ([]workload, error) {
	daemonSets, err := c.client.AppsV1().DaemonSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var workloads []workload
	for i := range daemonSets.Items {
		workloads = append(workloads, makeDaemonSetWorkload(&daemonSets.Items[i]))
	}

	return workloads, nil
}

func makeDaemonSetWorkload(daemonSet *apiapps.DaemonSet) workload {
	var status string
	objectMeta, daemonSetStatus := daemonSet.ObjectMeta, daemonSet.Status

	status = cluster.StatusStarted
	rollout := cluster.RolloutStatus{
		Desired:   daemonSetStatus.DesiredNumberScheduled,
		Updated:   daemonSetStatus.UpdatedNumberScheduled,
		Ready:     daemonSetStatus.NumberReady,
		Available: daemonSetStatus.NumberAvailable,
		Outdated:  daemonSetStatus.CurrentNumberScheduled - daemonSetStatus.UpdatedNumberScheduled,
		// TODO Add Messages after "TODO: Add valid condition types of a DaemonSet" fixed in
		// https://github.com/kubernetes/kubernetes/blob/f3e0750754ebeea4ea8e0d452cbaf55426751d12/pkg/apis/extensions/types.go#L434
	}

	if daemonSetStatus.ObservedGeneration >= objectMeta.Generation {
		// the definition has been updated; now let's see about the replicas
		status = cluster.StatusUpdating
		if rollout.Updated == rollout.Desired && rollout.Available == rollout.Desired && rollout.Outdated == 0 {
			status = cluster.StatusReady
		}
	}

	return workload{
		apiVersion:  "apps/v1",
		kind:        "DaemonSet",
		name:        daemonSet.ObjectMeta.Name,
		status:      status,
		rollout:     rollout,
		podTemplate: daemonSet.Spec.Template,
		k8sObject:   daemonSet}
}

/////////////////////////////////////////////////////////////////////////////
// apps/v1beta1 StatefulSet

type statefulSetKind struct{}

func (dk *statefulSetKind) getWorkload(c *Cluster, namespace, name string) (workload, error) {
	statefulSet, err := c.client.AppsV1().StatefulSets(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}

	return makeStatefulSetWorkload(statefulSet), nil
}

func (dk *statefulSetKind) getWorkloads(c *Cluster, namespace string) ([]workload, error) {
	statefulSets, err := c.client.AppsV1().StatefulSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var workloads []workload
	for i := range statefulSets.Items {
		workloads = append(workloads, makeStatefulSetWorkload(&statefulSets.Items[i]))
	}

	return workloads, nil
}

func makeStatefulSetWorkload(statefulSet *apiapps.StatefulSet) workload {
	var status string
	objectMeta, statefulSetStatus := statefulSet.ObjectMeta, statefulSet.Status

	status = cluster.StatusStarted
	rollout := cluster.RolloutStatus{
		Ready: statefulSetStatus.ReadyReplicas,
		// There is no Available parameter for statefulset, so use Ready instead
		Available: statefulSetStatus.ReadyReplicas,
		// TODO Add Messages after "TODO: Add valid condition types for Statefulsets." fixed in
		// https://github.com/kubernetes/kubernetes/blob/7f23a743e8c23ac6489340bbb34fa6f1d392db9d/pkg/apis/apps/types.go#L205
	}

	var specDesired int32
	if statefulSet.Spec.Replicas != nil {
		rollout.Desired = *statefulSet.Spec.Replicas
		specDesired = *statefulSet.Spec.Replicas
	}

	// rolling update
	if statefulSet.Spec.UpdateStrategy.Type == apiapps.RollingUpdateStatefulSetStrategyType &&
		statefulSet.Spec.UpdateStrategy.RollingUpdate != nil &&
		statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		// Desired for this partition: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#partitions
		desiredPartition := rollout.Desired - *statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition
		if desiredPartition >= 0 {
			rollout.Desired = desiredPartition
		} else {
			rollout.Desired = 0
		}
	}

	if statefulSetStatus.CurrentRevision != statefulSetStatus.UpdateRevision {
		// rollout in progress
		rollout.Updated = statefulSetStatus.UpdatedReplicas

	} else {
		// rollout complete
		rollout.Updated = statefulSetStatus.CurrentReplicas
	}

	rollout.Outdated = rollout.Desired - rollout.Updated

	if statefulSetStatus.ObservedGeneration >= objectMeta.Generation {
		// the definition has been updated; now let's see about the replicas
		status = cluster.StatusUpdating
		// for partition rolling update rollout.Ready might be >= rollout.Desired
		// because of rollout.Ready references to all ready pods (updated and outdated ones)
		// and rollout.Desired references to only desired pods for current partition
		// we check that all pods (updated and outdated ones) are ready
		if rollout.Updated == rollout.Desired && rollout.Ready == specDesired && rollout.Outdated == 0 {
			status = cluster.StatusReady
		}
	}

	return workload{
		apiVersion:  "apps/v1",
		kind:        "StatefulSet",
		name:        statefulSet.ObjectMeta.Name,
		status:      status,
		rollout:     rollout,
		podTemplate: statefulSet.Spec.Template,
		k8sObject:   statefulSet}
}

/////////////////////////////////////////////////////////////////////////////
// batch/v1beta1 CronJob

type cronJobKind struct{}

func (dk *cronJobKind) getWorkload(c *Cluster, namespace, name string) (workload, error) {
	cronJob, err := c.client.BatchV1beta1().CronJobs(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}

	return makeCronJobWorkload(cronJob), nil
}

func (dk *cronJobKind) getWorkloads(c *Cluster, namespace string) ([]workload, error) {
	cronJobs, err := c.client.BatchV1beta1().CronJobs(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var workloads []workload
	for i, _ := range cronJobs.Items {
		workloads = append(workloads, makeCronJobWorkload(&cronJobs.Items[i]))
	}

	return workloads, nil
}

func makeCronJobWorkload(cronJob *apibatch.CronJob) workload {
	return workload{
		apiVersion:  "batch/v1beta1",
		kind:        "CronJob",
		name:        cronJob.ObjectMeta.Name,
		status:      cluster.StatusReady,
		podTemplate: cronJob.Spec.JobTemplate.Spec.Template,
		k8sObject:   cronJob}
}

/////////////////////////////////////////////////////////////////////////////
// helm.integrations.flux.weave.works/v1alpha2 FluxHelmRelease

type fluxHelmReleaseKind struct{}

func (fhr *fluxHelmReleaseKind) getWorkload(c *Cluster, namespace, name string) (workload, error) {
	fluxHelmRelease, err := c.client.HelmV1alpha2().FluxHelmReleases(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}
	return makeFluxHelmReleaseWorkload(fluxHelmRelease), nil
}

func (fhr *fluxHelmReleaseKind) getWorkloads(c *Cluster, namespace string) ([]workload, error) {
	fluxHelmReleases, err := c.client.HelmV1alpha2().FluxHelmReleases(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var workloads []workload
	for _, f := range fluxHelmReleases.Items {
		workloads = append(workloads, makeFluxHelmReleaseWorkload(&f))
	}

	return workloads, nil
}

func makeFluxHelmReleaseWorkload(fluxHelmRelease *fhr_v1alpha2.FluxHelmRelease) workload {
	containers := createK8sFHRContainers(fluxHelmRelease.Spec.Values)

	podTemplate := apiv1.PodTemplateSpec{
		ObjectMeta: fluxHelmRelease.ObjectMeta,
		Spec: apiv1.PodSpec{
			Containers:       containers,
			ImagePullSecrets: []apiv1.LocalObjectReference{},
		},
	}

	return workload{
		apiVersion:  "helm.integrations.flux.weave.works/v1alpha2",
		kind:        "FluxHelmRelease",
		name:        fluxHelmRelease.ObjectMeta.Name,
		status:      fluxHelmRelease.Status.ReleaseStatus,
		podTemplate: podTemplate,
		k8sObject:   fluxHelmRelease,
	}
}

// createK8sContainers creates a list of k8s containers by
// interpreting the FluxHelmRelease resource. The interpretation is
// analogous to that in cluster/kubernetes/resource/fluxhelmrelease.go
func createK8sFHRContainers(values map[string]interface{}) []apiv1.Container {
	var containers []apiv1.Container
	_ = kresource.FindFluxHelmReleaseContainers(values, func(name string, image image.Ref, _ kresource.ImageSetter) error {
		containers = append(containers, apiv1.Container{
			Name:  name,
			Image: image.String(),
		})
		return nil
	})
	return containers
}

/////////////////////////////////////////////////////////////////////////////
// flux.weave.works/v1beta1 HelmRelease

type helmReleaseKind struct{}

func (hr *helmReleaseKind) getWorkload(c *Cluster, namespace, name string) (workload, error) {
	helmRelease, err := c.client.FluxV1beta1().HelmReleases(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}
	return makeHelmReleaseWorkload(helmRelease), nil
}

func (hr *helmReleaseKind) getWorkloads(c *Cluster, namespace string) ([]workload, error) {
	helmReleases, err := c.client.FluxV1beta1().HelmReleases(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var workloads []workload
	for _, f := range helmReleases.Items {
		workloads = append(workloads, makeHelmReleaseWorkload(&f))
	}

	return workloads, nil
}

func makeHelmReleaseWorkload(helmRelease *fhr_v1beta1.HelmRelease) workload {
	containers := createK8sFHRContainers(helmRelease.Spec.Values)

	podTemplate := apiv1.PodTemplateSpec{
		ObjectMeta: helmRelease.ObjectMeta,
		Spec: apiv1.PodSpec{
			Containers:       containers,
			ImagePullSecrets: []apiv1.LocalObjectReference{},
		},
	}

	return workload{
		apiVersion:  "flux.weave.works/v1beta1",
		kind:        "HelmRelease",
		name:        helmRelease.ObjectMeta.Name,
		status:      helmRelease.Status.ReleaseStatus,
		podTemplate: podTemplate,
		k8sObject:   helmRelease,
	}
}
