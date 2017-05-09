package kubernetes

import (
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

const (
	policyPrefix = "flux.weave.works/"
)

func (c *Cluster) UpdatePolicies(in []byte, update policy.Update) ([]byte, error) {
	return updateAnnotations(in, func(a map[string]string) map[string]string {
		for _, policy := range update.Add {
			a[policyPrefix+string(policy)] = "true"
		}
		for _, policy := range update.Remove {
			delete(a, policyPrefix+string(policy))
		}
		return a
	})
}

func updateAnnotations(def []byte, f func(map[string]string) map[string]string) ([]byte, error) {
	manifest, err := parseManifest(def)
	if err != nil {
		return nil, err
	}
	newAnnotations := f(manifest.Metadata.AnnotationsOrNil())

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
	annotationsRE := regexp.MustCompile(`(?m:\n  annotations:\s*(?:#.*)*(?:\n    .*)*$)`)
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
				Containers []Container `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
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

func (c *Cluster) ServicesWithPolicy(root string, policy policy.Policy) (flux.ServiceIDSet, error) {
	all, err := c.FindDefinedServices(root)
	if err != nil {
		return nil, err
	}
	result := flux.ServiceIDSet{}
	for serviceID, paths := range all {
		if len(paths) != 1 {
			continue
		}

		def, err := ioutil.ReadFile(paths[0])
		if err != nil {
			return nil, err
		}

		p, err := policiesFrom(def)
		if err != nil {
			return nil, err
		}
		if p.Contains(policy) {
			result.Add([]flux.ServiceID{serviceID})
		}
	}
	return result, nil
}

func policiesFrom(def []byte) (policy.PolicySet, error) {
	manifest, err := parseManifest(def)
	if err != nil {
		return nil, err
	}

	var policies policy.PolicySet
	for k, v := range manifest.Metadata.AnnotationsOrNil() {
		if !strings.HasPrefix(k, policyPrefix) {
			continue
		}
		if v != "true" {
			continue
		}
		policies = policies.Add(policy.Parse(strings.TrimPrefix(k, policyPrefix)))
	}
	return policies, nil
}
