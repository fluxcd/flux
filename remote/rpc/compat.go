package rpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api/v10"
	"github.com/weaveworks/flux/api/v11"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

func requireWorkloadSpecKinds(ss update.ResourceSpec, kinds []string) error {
	id, err := ss.AsID()
	if err != nil {
		return nil
	}

	_, kind, _ := id.Components()
	if !contains(kinds, kind) {
		return fmt.Errorf("Unsupported resource kind: %s", kind)
	}

	return nil
}

func requireWorkloadIDKinds(id flux.ResourceID, kinds []string) error {
	_, kind, _ := id.Components()
	if !contains(kinds, kind) {
		return fmt.Errorf("Unsupported resource kind: %s", kind)
	}

	return nil
}

func requireSpecKinds(s update.Spec, kinds []string) error {
	switch s := s.Spec.(type) {
	case policy.Updates:
		for id, _ := range s {
			_, kind, _ := id.Components()
			if !contains(kinds, kind) {
				return fmt.Errorf("Unsupported resource kind: %s", kind)
			}
		}
	case update.ReleaseImageSpec:
		for _, ss := range s.WorkloadSpecs {
			if err := requireWorkloadSpecKinds(ss, kinds); err != nil {
				return err
			}
		}
		for _, id := range s.Excludes {
			_, kind, _ := id.Components()
			if !contains(kinds, kind) {
				return fmt.Errorf("Unsupported resource kind: %s", kind)
			}
		}
	}
	return nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// listWorkloadsRolloutStatus polyfills the rollout status.
func listWorkloadsRolloutStatus(ss []v6.WorkloadStatus) {
	for i := range ss {
		// Polyfill for daemons that list pod information in status ('X out of N updated')
		if n, _ := fmt.Sscanf(ss[i].Status, "%d out of %d updated", &ss[i].Rollout.Updated, &ss[i].Rollout.Desired); n == 2 {
			// Daemons on an earlier version determined the workload to be ready if updated == desired.
			//
			// Technically, 'updated' does *not* yet mean these pods are ready and accepting traffic. There
			// can still be outdated pods that serve requests. The new way of determining the end of a rollout
			// is to make sure the desired count equals to the number of 'available' pods and zero outdated
			// pods. To make older daemons reach a "rollout is finished" state we set 'available', 'ready',
			// and 'updated' all to the same count.
			ss[i].Rollout.Ready = ss[i].Rollout.Updated
			ss[i].Rollout.Available = ss[i].Rollout.Updated
			ss[i].Status = cluster.StatusUpdating
		}
	}
}

type listWorkloadsWithoutOptionsClient interface {
	ListWorkloads(ctx context.Context, namespace string) ([]v6.WorkloadStatus, error)
}

// listWorkloadsWithOptions polyfills the ListWorkloadsWithOptions()
// introduced in v11 by removing unwanted resources after fetching
// all the workloads.
func listWorkloadsWithOptions(ctx context.Context, p listWorkloadsWithoutOptionsClient, opts v11.ListWorkloadsOptions, supportedKinds []string) ([]v6.WorkloadStatus, error) {
	if opts.Namespace != "" && len(opts.Workloads) > 0 {
		return nil, errors.New("cannot filter by 'namespace' and 'workloads' at the same time")
	}
	if len(supportedKinds) > 0 {
		for _, svc := range opts.Workloads {
			if err := requireWorkloadIDKinds(svc, supportedKinds); err != nil {
				return nil, remote.UnsupportedResourceKind(err)
			}
		}
	}

	all, err := p.ListWorkloads(ctx, opts.Namespace)
	listWorkloadsRolloutStatus(all)
	if err != nil {
		return nil, err
	}
	if len(opts.Workloads) == 0 {
		return all, nil
	}

	// Polyfill the workload IDs filter
	want := map[flux.ResourceID]struct{}{}
	for _, svc := range opts.Workloads {
		want[svc] = struct{}{}
	}
	var workloads []v6.WorkloadStatus
	for _, svc := range all {
		if _, ok := want[svc.ID]; ok {
			workloads = append(workloads, svc)
		}
	}
	return workloads, nil
}

type listImagesWithoutOptionsClient interface {
	ListWorkloads(ctx context.Context, namespace string) ([]v6.WorkloadStatus, error)
	ListImages(ctx context.Context, spec update.ResourceSpec) ([]v6.ImageStatus, error)
}

// listImagesWithOptions is called by ListImagesWithOptions so we can use an
// interface to dispatch .ListImages() and .ListWorkloads() to the correct
// API version.
func listImagesWithOptions(ctx context.Context, client listImagesWithoutOptionsClient, opts v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	statuses, err := client.ListImages(ctx, opts.Spec)
	if err != nil {
		return statuses, err
	}

	var ns string
	if opts.Spec != update.ResourceSpecAll {
		resourceID, err := opts.Spec.AsID()
		if err != nil {
			return statuses, err
		}
		ns, _, _ = resourceID.Components()
	}
	workloads, err := client.ListWorkloads(ctx, ns)
	if err != nil {
		return statuses, err
	}

	policyMap := map[flux.ResourceID]map[string]string{}
	for _, workload := range workloads {
		policyMap[workload.ID] = workload.Policies
	}

	// Polyfill container fields from v10
	for i, status := range statuses {
		for j, container := range status.Containers {
			var p policy.Set
			if policies, ok := policyMap[status.ID]; ok {
				p = policy.Set{}
				for k, v := range policies {
					p[policy.Policy(k)] = v
				}
			}
			tagPattern := policy.GetTagPattern(p, container.Name)
			// Create a new container using the same function used in v10
			newContainer, err := v6.NewContainer(container.Name, update.ImageInfos(container.Available), container.Current, tagPattern, opts.OverrideContainerFields)
			if err != nil {
				return statuses, err
			}
			statuses[i].Containers[j] = newContainer
		}
	}

	return statuses, nil
}
