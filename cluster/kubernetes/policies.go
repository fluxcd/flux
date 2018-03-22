package kubernetes

import (
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/policy"
)

func (m *Manifests) UpdatePolicies(in []byte, update policy.Update) ([]byte, error) {
	tagAll, _ := update.Add.Get(policy.TagAll)
	return updateAnnotations(in, tagAll, func(a map[string]string) map[string]string {
		for p, v := range update.Add {
			if p == policy.TagAll {
				continue
			}
			a[resource.PolicyPrefix+string(p)] = v
		}
		for p, _ := range update.Remove {
			delete(a, resource.PolicyPrefix+string(p))
		}
		return a
	})
}

func updateAnnotations(def []byte, tagAll string, f func(map[string]string) map[string]string) ([]byte, error) {
	manifest, err := parseManifest(def)
	if err != nil {
		return nil, err
	}
	annotations := manifest.Metadata.AnnotationsOrNil()
	if tagAll != "" {
		containers := manifest.Spec.Template.Spec.Containers
		for _, c := range containers {
			p := resource.PolicyPrefix + string(policy.TagPrefix(c.Name))
			if tagAll != "glob:*" {
				annotations[p] = tagAll
			} else {
				delete(annotations, p)
			}
		}
	}
	newAnnotations := f(annotations)

	// Write the new annotations back into the manifest
	// Generate a fragment of the new annotations.
	var fragment string
	if len(newAnnotations) > 0 {
		fragmentB, err := yaml.Marshal(map[string]map[string]string{
			"annotations": newAnnotations,
		})
		if err != nil {
			return nil, err
		}

		fragment = string(fragmentB)

		// Remove the last newline, so it fits in better
		fragment = strings.TrimSuffix(fragment, "\n")

		// indent the fragment 2 spaces
		fragment = regexp.MustCompile(`(.+)`).ReplaceAllString(fragment, "  $1")

		// Add a newline if it's not blank
		if len(fragment) > 0 {
			fragment = "\n" + fragment
		}
	}

	// Find where to insert the fragment.
	// TODO: This should handle potentially different indentation.
	// TODO: There's probably a more elegant regex-ey way to do this in one pass.
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

type Manifest struct {
	Metadata Metadata `yaml:"metadata"`
	Spec     struct {
		Template struct {
			Spec struct {
				InitContainers []Container `yaml:"initContainers"`
				Containers []Container `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
		JobTemplate struct {
			Spec struct {
				Template struct {
					Spec struct {
						Containers []Container `yaml:"containers"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		} `yaml:"jobTemplate"`
	} `yaml:"spec"`
}

func (m Metadata) AnnotationsOrNil() map[string]string {
	if m.Annotations == nil {
		return map[string]string{}
	}
	return m.Annotations
}

type Metadata struct {
	Name        string            `yaml:"name"`
	Annotations map[string]string `yaml:"annotations"`
}

type Container struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
}

func parseManifest(def []byte) (Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(def, &m); err != nil {
		return m, errors.Wrap(err, "decoding annotations")
	}
	return m, nil
}

func (m *Manifests) ServicesWithPolicies(root string) (policy.ResourceMap, error) {
	all, err := m.FindDefinedServices(root)
	if err != nil {
		return nil, err
	}

	result := map[flux.ResourceID]policy.Set{}
	err = iterateManifests(all, func(s flux.ResourceID, m Manifest) error {
		ps, err := policiesFrom(m)
		if err != nil {
			return err
		}
		result[s] = ps
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func iterateManifests(services map[flux.ResourceID][]string, f func(flux.ResourceID, Manifest) error) error {
	for serviceID, paths := range services {
		if len(paths) != 1 {
			continue
		}

		def, err := ioutil.ReadFile(paths[0])
		if err != nil {
			return err
		}
		manifest, err := parseManifest(def)
		if err != nil {
			return err
		}

		if err = f(serviceID, manifest); err != nil {
			return err
		}
	}
	return nil
}

func policiesFrom(m Manifest) (policy.Set, error) {
	var policies policy.Set
	for k, v := range m.Metadata.AnnotationsOrNil() {
		if !strings.HasPrefix(k, resource.PolicyPrefix) {
			continue
		}
		p := policy.Policy(strings.TrimPrefix(k, resource.PolicyPrefix))
		if policy.Boolean(p) {
			if v != "true" {
				continue
			}
			policies = policies.Add(p)
		} else {
			policies = policies.Set(p, v)
		}
	}
	return policies, nil
}
