package rpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/remote"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

func requireServiceSpecKinds(ss update.ResourceSpec, kinds []string) error {
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

func requireServiceIDKinds(id resource.ID, kinds []string) error {
	_, kind, _ := id.Components()
	if !contains(kinds, kind) {
		return fmt.Errorf("Unsupported resource kind: %s", kind)
	}

	return nil
}

func requireSpecKinds(s update.Spec, kinds []string) error {
	switch s := s.Spec.(type) {
	case resource.PolicyUpdates:
		for id, _ := range s {
			_, kind, _ := id.Components()
			if !contains(kinds, kind) {
				return fmt.Errorf("Unsupported resource kind: %s", kind)
			}
		}
	case update.ReleaseImageSpec:
		for _, ss := range s.ServiceSpecs {
			if err := requireServiceSpecKinds(ss, kinds); err != nil {
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

// listServicesRolloutStatus polyfills the rollout status.
func listServicesRolloutStatus(ss []v6.ControllerStatus) {
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

type listServicesWithoutOptionsClient interface {
	ListServices(ctx context.Context, namespace string) ([]v6.ControllerStatus, error)
}

// listServicesWithOptions polyfills the ListServiceWithOptions()
// introduced in v11 by removing unwanted resources after fetching
// all the services.
func listServicesWithOptions(ctx context.Context, p listServicesWithoutOptionsClient, opts v11.ListServicesOptions, supportedKinds []string) ([]v6.ControllerStatus, error) {
	if opts.Namespace != "" && len(opts.Services) > 0 {
		return nil, errors.New("cannot filter by 'namespace' and 'services' at the same time")
	}
	if len(supportedKinds) > 0 {
		for _, svc := range opts.Services {
			if err := requireServiceIDKinds(svc, supportedKinds); err != nil {
				return nil, remote.UnsupportedResourceKind(err)
			}
		}
	}

	all, err := p.ListServices(ctx, opts.Namespace)
	listServicesRolloutStatus(all)
	if err != nil {
		return nil, err
	}
	if len(opts.Services) == 0 {
		return all, nil
	}

	// Polyfill the service IDs filter
	want := map[resource.ID]struct{}{}
	for _, svc := range opts.Services {
		want[svc] = struct{}{}
	}
	var controllers []v6.ControllerStatus
	for _, svc := range all {
		if _, ok := want[svc.ID]; ok {
			controllers = append(controllers, svc)
		}
	}
	return controllers, nil
}

type listImagesWithoutOptionsClient interface {
	ListServices(ctx context.Context, namespace string) ([]v6.ControllerStatus, error)
	ListImages(ctx context.Context, spec update.ResourceSpec) ([]v6.ImageStatus, error)
}

type alreadySorted update.SortedImageInfos

func (infos alreadySorted) Images() []image.Info {
	return []image.Info(infos)
}

func (infos alreadySorted) SortedImages(_ policy.Pattern) update.SortedImageInfos {
	return update.SortedImageInfos(infos)
}

// listImagesWithOptions is called by ListImagesWithOptions so we can use an
// interface to dispatch .ListImages() and .ListServices() to the correct
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
	services, err := client.ListServices(ctx, ns)
	if err != nil {
		return statuses, err
	}

	policyMap := map[resource.ID]map[string]string{}
	for _, service := range services {
		policyMap[service.ID] = service.Policies
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
			newContainer, err := v6.NewContainer(container.Name, alreadySorted(container.Available), container.Current, tagPattern, opts.OverrideContainerFields)
			if err != nil {
				return statuses, err
			}
			statuses[i].Containers[j] = newContainer
		}
	}

	return statuses, nil
}
