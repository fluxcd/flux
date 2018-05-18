package kubernetes

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

func (m *Manifests) UpdatePolicies(path string, id flux.ResourceID, update policy.Update) error {
	return updateManifest(path, id, func(def []byte) ([]byte, error) {
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
				if tagAll == "glob:*" {
					del = del.Add(policy.TagPrefix(container.Name))
				} else {
					add = add.Set(policy.TagPrefix(container.Name), tagAll)
				}
			}
		}

		return updateAnnotations(def, id, func(old map[string]string) {
			for k, v := range add {
				old[kresource.PolicyPrefix+string(k)] = v
			}
			for k := range del {
				delete(old, kresource.PolicyPrefix+string(k))
			}
		})
	})
}

func updateAnnotations(def []byte, id flux.ResourceID, f func(map[string]string)) ([]byte, error) {
	annotations, err := extractAnnotations(def)
	if err != nil {
		return nil, err
	}
	f(annotations)

	// Write the new annotations back into the manifest
	// Generate a fragment of the new annotations.
	var fragment string
	if len(annotations) > 0 {
		fragmentB, err := yaml.Marshal(map[string]map[string]string{
			"annotations": annotations,
		})
		if err != nil {
			return nil, err
		}

		fragment = string(fragmentB)

		// Remove the last newline, so it fits in better
		fragment = strings.TrimSuffix(fragment, "\n")

		// indent the fragment 2 spaces
		// TODO: delete all regular expressions which are used to modify YAML.
		// See #1019. Modifying this is not recommended.
		fragment = regexp.MustCompile(`(.+)`).ReplaceAllString(fragment, "  $1")

		// Add a newline if it's not blank
		if len(fragment) > 0 {
			fragment = "\n" + fragment
		}
	}

	// Find where to insert the fragment.
	// TODO: delete all regular expressions which are used to modify YAML.
	// See #1019. Modifying this is not recommended.
	replaced := false
	annotationsRE := regexp.MustCompile(`(?m:\n  annotations:\s*(?:#.*)*(?:\n    .*|\n)*$)`)
	newDef := annotationsRE.ReplaceAllStringFunc(string(def), func(found string) string {
		if !replaced {
			replaced = true
			return fragment
		}
		return found
	})
	if !replaced {
		metadataRE := multilineRE(`(metadata:\s*(?:#.*)*)`)
		newDef = metadataRE.ReplaceAllStringFunc(string(def), func(found string) string {
			if !replaced {
				replaced = true
				f := found + fragment
				return f
			}
			return found
		})
	}
	if !replaced {
		return nil, errors.New("Could not update resource annotations")
	}

	return []byte(newDef), err
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

func (m *Manifests) ServicesWithPolicies(root string) (policy.ResourceMap, error) {
	resources, err := m.LoadManifests(root, root)
	if err != nil {
		return nil, err
	}
	result := policy.ResourceMap{}
	for _, res := range resources {
		result[res.ResourceID()] = res.Policy()
	}
	return result, nil
}
