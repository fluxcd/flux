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

func (m *Manifests) UpdatePolicies(original io.Reader, id flux.ResourceID, update policy.Update) (io.Reader, error) {
	add, del := update.Add, update.Remove

	// We may be sent the pseudo-policy `policy.TagAll`, which means
	// apply this filter to all containers. To do so, we need to know
	// what all the containers are.
	if tagAll, ok := update.Add.Get(policy.TagAll); ok {
		add = add.Without(policy.TagAll)
		copy := &bytes.Buffer{}
		tee := io.TeeReader(original, copy)
		containers, err := extractContainers(tee, id)
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
		original = copy
	}

	return updateAnnotations(original, id, func(old map[string]string) {
		for k, v := range add {
			old[kresource.PolicyPrefix+string(k)] = v
		}
		for k := range del {
			delete(old, kresource.PolicyPrefix+string(k))
		}
	})
}

func updateAnnotations(original io.Reader, id flux.ResourceID, f func(map[string]string)) (io.Reader, error) {
	copy := &bytes.Buffer{}
	tee := io.TeeReader(original, copy)
	annotations, err := extractAnnotations(tee)
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

	def, err := ioutil.ReadAll(copy)
	if err != nil {
		return nil, err
	}
	newDef := annotationsRE.ReplaceAllFunc(def, func(found []byte) []byte {
		if !replaced {
			replaced = true
			return []byte(fragment)
		}
		return found
	})
	if !replaced {
		metadataRE := multilineRE(`(metadata:\s*(?:#.*)*)`)
		newDef = metadataRE.ReplaceAllFunc(def, func(found []byte) []byte {
			if !replaced {
				replaced = true
				f := append(found, []byte(fragment)...)
				return f
			}
			return found
		})
	}
	if !replaced {
		return nil, errors.New("Could not update resource annotations")
	}

	return bytes.NewBuffer(newDef), nil
}

type manifest struct {
	Metadata struct {
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
}

func extractAnnotations(def io.Reader) (map[string]string, error) {
	var m manifest
	if err := yaml.NewDecoder(def).Decode(&m); err != nil {
		return nil, errors.Wrap(err, "decoding manifest for annotations")
	}
	if m.Metadata.Annotations == nil {
		return map[string]string{}, nil
	}
	return m.Metadata.Annotations, nil
}

func extractContainers(def io.Reader, id flux.ResourceID) ([]resource.Container, error) {
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
