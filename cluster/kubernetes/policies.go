package kubernetes

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

type PolicyTranslator struct{}

func (pt *PolicyTranslator) GetAnnotationChangesForPolicyUpdate(workload resource.Workload, update policy.Update) ([]cluster.AnnotationChange, error) {
	add, del := update.Add, update.Remove
	// We may be sent the pseudo-policy `policy.TagAll`, which means
	// apply this filter to all containers. To do so, we need to know
	// what all the containers are.
	if tagAll, ok := update.Add.Get(policy.TagAll); ok {
		add = add.Without(policy.TagAll)
		for _, container := range workload.Containers() {
			if tagAll == policy.PatternAll.String() {
				del = del.Add(policy.TagPrefix(container.Name))
			} else {
				add = add.Set(policy.TagPrefix(container.Name), tagAll)
			}
		}
	}

	result := []cluster.AnnotationChange{}
	for pol, val := range add {
		if policy.Tag(pol) && !policy.NewPattern(val).Valid() {
			return nil, fmt.Errorf("invalid tag pattern: %q", val)
		}
		valCopy := val
		result = append(result, cluster.AnnotationChange{kresource.PolicyPrefix + string(pol), &valCopy})
	}
	for pol, _ := range del {
		result = append(result, cluster.AnnotationChange{kresource.PolicyPrefix + string(pol), nil})
	}
	return result, nil
}

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
	changes, err := (&PolicyTranslator{}).GetAnnotationChangesForPolicyUpdate(workload, update)
	var args []string
	for _, change := range changes {
		value := ""
		if change.AnnotationValue != nil {
			value = *change.AnnotationValue
		}
		args = append(args, fmt.Sprintf("%s=%s", change.AnnotationKey, value))
	}
	ns, kind, name := id.Components()
	return (KubeYAML{}).Annotate(def, ns, kind, name, args...)
}
