package kubernetes

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

func (m *manifests) UpdateWorkloadPolicies(def []byte, id flux.ResourceID, update policy.Update) ([]byte, error) {
	resources, err := m.ParseManifest(def, "stdin")
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
	if err != nil {
		return nil, err
	}

	changes, err := resource.ChangesForPolicyUpdate(workload, update)
	if err != nil {
		return nil, err
	}

	var args []string
	for k, v := range changes {
		args = append(args, fmt.Sprintf("%s%s=%s", kresource.PolicyPrefix, k, v))
	}

	ns, kind, name := id.Components()
	return (KubeYAML{}).Annotate(def, ns, kind, name, args...)
}
