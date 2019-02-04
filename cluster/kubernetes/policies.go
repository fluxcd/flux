package kubernetes

import (
	"fmt"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

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
		containers, err := extractContainers(def, id)
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

type manifest struct {
	Metadata struct {
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
}

func extractAnnotations(def []byte) (map[string]string, error) {
	var m manifest
	if err := yaml.Unmarshal(def, &m); err != nil {
		return nil, errors.Wrap(err, "decoding manifest for annotations")
	}
	if m.Metadata.Annotations == nil {
		return map[string]string{}, nil
	}
	return m.Metadata.Annotations, nil
}

func extractContainers(def []byte, id flux.ResourceID) ([]resource.Container, error) {
	resources, err := kresource.ParseMultidoc(def, "stdin")
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
