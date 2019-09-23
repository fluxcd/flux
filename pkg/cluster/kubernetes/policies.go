package kubernetes

import (
	"fmt"

	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
	"github.com/fluxcd/flux/pkg/resource"
)

func (m *manifests) UpdateWorkloadPolicies(def []byte, id resource.ID, update resource.PolicyUpdate) ([]byte, error) {
	resources, err := m.ParseManifest(def, "stdin")
	if err != nil {
		return nil, err
	}
	res, ok := resources[id.String()]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", id.String())
	}

	// This is the Kubernetes manifests implementation; panic if it's
	// not returning `KubeManifest`s.
	kres := res.(kresource.KubeManifest)

	workload, ok := res.(resource.Workload)
	if !ok {
		return nil, fmt.Errorf("resource %s does not have containers", id.String())
	}
	if err != nil {
		return nil, err
	}

	changes, err := resource.ChangesForPolicyUpdate(workload, update)
	if err != nil {
		return nil, err
	}

	var args []string
	for k, v := range changes {
		annotation, ok := kres.PolicyAnnotationKey(k)
		if !ok {
			annotation = fmt.Sprintf("%s%s", kresource.PolicyPrefix, k)
		}
		args = append(args, fmt.Sprintf("%s=%s", annotation, v))
	}

	ns, kind, name := id.Components()
	return (KubeYAML{}).Annotate(def, ns, kind, name, args...)
}
