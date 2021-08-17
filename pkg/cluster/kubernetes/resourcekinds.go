package kubernetes

import (
	"context"
	"strings"

	hr_v1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"
	apiapps "k8s.io/api/apps/v1"
	apibatch "k8s.io/api/batch/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/flux/pkg/cluster"
	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
)

// AntecedentAnnotation is an annotation on a resource indicating that
// the cause of that resource (indirectly, via a Helm release) is a
// HelmRelease. We use this rather than the `OwnerReference` type
// built into Kubernetes so that there are no garbage-collection
// implications. The value is expected to be a serialised
// `resource.ID`.
const AntecedentAnnotation = hr_v1.AntecedentAnnotation

/////////////////////////////////////////////////////////////////////////////
// Kind registry

type resourceKind interface {
	getWorkload(ctx context.Context, c *Cluster, namespace, name string) (workload, error)
	getWorkloads(ctx context.Context, c *Cluster, namespace string) ([]workload, error)
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
}

type workload struct {
	k8sObject
	status      string
	rollout     cluster.RolloutStatus
	syncError   error
	podTemplate apiv1.PodTemplateSpec
}

func (w workload) toClusterWorkload(resourceID resource.ID) cluster.Workload {
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

	var antecedent resource.ID
	if ante, ok := w.GetAnnotations()[AntecedentAnnotation]; ok {
		id, err := resource.ParseID(ante)
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

func (dk *deploymentKind) getWorkload(ctx context.Context, c *Cluster, namespace, name string) (workload, error) {
	if err := ctx.Err(); err != nil {
		return workload{}, err
	}
	deployment, err := c.client.AppsV1().Deployments(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}

	return makeDeploymentWorkload(deployment), nil
}

func (dk *deploymentKind) getWorkloads(ctx context.Context, c *Cluster, namespace string) ([]workload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deployments, err := c.client.AppsV1().Deployments(namespace).List(context.TODO(), meta_v1.ListOptions{})
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
	// apiVersion & kind must be set, since TypeMeta is not populated
	deployment.APIVersion = "apps/v1"
	deployment.Kind = "Deployment"
	return workload{
		status:      status,
		rollout:     rollout,
		podTemplate: deployment.Spec.Template,
		k8sObject:   deployment}
}

/////////////////////////////////////////////////////////////////////////////
// extensions/v1beta daemonset

type daemonSetKind struct{}

func (dk *daemonSetKind) getWorkload(ctx context.Context, c *Cluster, namespace, name string) (workload, error) {
	if err := ctx.Err(); err != nil {
		return workload{}, err
	}
	daemonSet, err := c.client.AppsV1().DaemonSets(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}

	return makeDaemonSetWorkload(daemonSet), nil
}

func (dk *daemonSetKind) getWorkloads(ctx context.Context, c *Cluster, namespace string) ([]workload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	daemonSets, err := c.client.AppsV1().DaemonSets(namespace).List(context.TODO(), meta_v1.ListOptions{})
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

	// apiVersion & kind must be set, since TypeMeta is not populated
	daemonSet.APIVersion = "apps/v1"
	daemonSet.Kind = "DaemonSet"
	return workload{
		status:      status,
		rollout:     rollout,
		podTemplate: daemonSet.Spec.Template,
		k8sObject:   daemonSet}
}

/////////////////////////////////////////////////////////////////////////////
// apps/v1beta1 StatefulSet

type statefulSetKind struct{}

func (dk *statefulSetKind) getWorkload(ctx context.Context, c *Cluster, namespace, name string) (workload, error) {
	if err := ctx.Err(); err != nil {
		return workload{}, err
	}
	statefulSet, err := c.client.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}

	return makeStatefulSetWorkload(statefulSet), nil
}

func (dk *statefulSetKind) getWorkloads(ctx context.Context, c *Cluster, namespace string) ([]workload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	statefulSets, err := c.client.AppsV1().StatefulSets(namespace).List(context.TODO(), meta_v1.ListOptions{})
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

	// apiVersion & kind must be set, since TypeMeta is not populated
	statefulSet.APIVersion = "apps/v1"
	statefulSet.Kind = "StatefulSet"
	return workload{
		status:      status,
		rollout:     rollout,
		podTemplate: statefulSet.Spec.Template,
		k8sObject:   statefulSet}
}

/////////////////////////////////////////////////////////////////////////////
// batch/v1beta1 CronJob

type cronJobKind struct{}

func (dk *cronJobKind) getWorkload(ctx context.Context, c *Cluster, namespace, name string) (workload, error) {
	if err := ctx.Err(); err != nil {
		return workload{}, err
	}
	cronJob, err := c.client.BatchV1beta1().CronJobs(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		return workload{}, err
	}

	return makeCronJobWorkload(cronJob), nil
}

func (dk *cronJobKind) getWorkloads(ctx context.Context, c *Cluster, namespace string) ([]workload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cronJobs, err := c.client.BatchV1beta1().CronJobs(namespace).List(context.TODO(), meta_v1.ListOptions{})
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
	cronJob.APIVersion = "batch/v1beta1"
	cronJob.Kind = "CronJob"
	return workload{
		status:      cluster.StatusReady,
		podTemplate: cronJob.Spec.JobTemplate.Spec.Template,
		k8sObject:   cronJob}
}

/////////////////////////////////////////////////////////////////////////////
// flux.weave.works/v1beta1 HelmRelease

type helmReleaseKind struct{}

// getWorkload attempts to resolve a HelmRelease
func (hr *helmReleaseKind) getWorkload(ctx context.Context, c *Cluster, namespace, name string) (workload, error) {
	if err := ctx.Err(); err != nil {
		return workload{}, err
	}
	if helmRelease, err := c.client.HelmV1().HelmReleases(namespace).Get(name, meta_v1.GetOptions{}); err == nil {
		return makeHelmReleaseStableWorkload(helmRelease), err
	} else {
		return workload{}, err
	}
}

// getWorkloads collects v1 HelmRelease workloads
func (hr *helmReleaseKind) getWorkloads(ctx context.Context, c *Cluster, namespace string) ([]workload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	names := make(map[string]bool, 0)
	workloads := make([]workload, 0)
	if helmReleases, err := c.client.HelmV1().HelmReleases(namespace).List(meta_v1.ListOptions{}); err == nil {
		for i, _ := range helmReleases.Items {
			workload := makeHelmReleaseStableWorkload(&helmReleases.Items[i])
			workloads = append(workloads, workload)
			names[workload.GetName()] = true
		}
	} else {
		return nil, err
	}

	return workloads, nil
}

func makeHelmReleaseStableWorkload(helmRelease *hr_v1.HelmRelease) workload {
	containers := createK8sHRContainers(helmRelease.ObjectMeta.Annotations, helmRelease.GetValues())

	podTemplate := apiv1.PodTemplateSpec{
		ObjectMeta: helmRelease.ObjectMeta,
		Spec: apiv1.PodSpec{
			Containers:       containers,
			ImagePullSecrets: []apiv1.LocalObjectReference{},
		},
	}
	// apiVersion & kind must be set, since TypeMeta is not populated
	helmRelease.APIVersion = "helm.fluxcd.io/v1"
	helmRelease.Kind = "HelmRelease"
	return workload{
		status:      helmRelease.Status.ReleaseStatus,
		podTemplate: podTemplate,
		k8sObject:   helmRelease,
	}
}

// createK8sContainers creates a list of k8s containers by
// interpreting the HelmRelease resource.
func createK8sHRContainers(annotations map[string]string, values map[string]interface{}) []apiv1.Container {
	var containers []apiv1.Container
	kresource.FindHelmReleaseContainers(annotations, values, func(name string, image image.Ref, _ kresource.ImageSetter) error {
		containers = append(containers, apiv1.Container{
			Name:  name,
			Image: image.String(),
		})
		return nil
	})
	return containers
}
