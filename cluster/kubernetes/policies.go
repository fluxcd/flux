package kubernetes

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

func (m *Manifests) UpdatePolicies(def []byte, id flux.ResourceID, update policy.Update) ([]byte, error) {
	ns, kind, name := id.Components()
	add, del := update.Add, update.Remove

	// We may be sent the pseudo-policy `policy.TagAll`, which means
	// apply this filter to all containers. To do so, we need to know
	// what all the containers are.
	if tagAll, ok := update.Add.Get(policy.TagAll); ok {
		add = add.Without(policy.TagAll)
		containers, err := m.extractWorkloadContainers(def, id)
		if err != nil {
			return nil, err
		}

		for _, container := range containers {
			if tagAll == policy.PatternAll.String() {
				del = del.Add(policy.TagPrefix(container.Name))
			} else {
				add = add.Set(policy.TagPrefix(container.Name), tagAll)
			}
		}
	}

	var args []string
	for pol, val := range add {
		if policy.Tag(pol) && !policy.NewPattern(val).Valid() {
			return nil, fmt.Errorf("invalid tag pattern: %q", val)
		}
		args = append(args, fmt.Sprintf("%s%s=%s", kresource.PolicyPrefix, pol, val))
	}
	for pol, _ := range del {
		args = append(args, fmt.Sprintf("%s%s=", kresource.PolicyPrefix, pol))
	}

	return (KubeYAML{}).Annotate(def, ns, kind, name, args...)
}

func (m *Manifests) extractWorkloadContainers(def []byte, id flux.ResourceID) ([]resource.Container, error) {
	kresources, err := kresource.ParseMultidoc(def, "stdin")
	if err != nil {
		return nil, err
	}
	// Note: setEffectiveNamespaces() won't work for CRD instances whose CRD is yet to be created
	// (due to the CRD not being present in kresources).
	// We could get out of our way to fix this (or give a better error) but:
	// 1. With the exception of HelmReleases CRD instances are not workloads anyways.
	// 2. The problem is eventually fixed by the first successful sync.
	resources, err := setEffectiveNamespaces(kresources, m.Namespacer)
	if err != nil {
		return nil, err
	}
	res, ok := resources[id.String()]
	if !ok {
		return nil, errors.New("resource " + id.String() + " not found")
	}
	workload, ok := res.(resource.Workload)
	if !ok {
		return nil, errors.New("resource " + id.String() + " does not have containers")
	}
	return workload.Containers(), nil
}
