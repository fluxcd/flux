package kubernetes

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/policy"
)

func (m *Manifests) UpdatePolicies(in []byte, serviceID flux.ResourceID, update policy.Update) ([]byte, error) {
	tagAll, _ := update.Add.Get(policy.TagAll)

	return updateAnnotations(in, serviceID, tagAll, func(a map[string]string) map[string]string {
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

func updateAnnotations(def []byte, serviceID flux.ResourceID, tagAll string, f func(map[string]string) map[string]string) ([]byte, error) {
	manifest, err := parseManifest(def)
	if err != nil {
		return nil, err
	}

	str := string(def)
	isList := manifest.Kind == "List"
	var annotations map[string]string
	var containers []resource.Container
	var annotationsExpression string
	var metadataExpression string

	if isList {
		var l resource.List
		err := yaml.Unmarshal(def, &l)

		if err != nil {
			return nil, err
		}

		// find the item we are trying to update in the List
		for _, item := range l.Items {
			if item.ResourceID().String() == serviceID.String() {
				annotations = item.Metadata.AnnotationsOrNil()
				containers = item.Spec.Template.Spec.Containers
				break
			}
		}
		// Grab the annotations from the metadata block.
		annotationsExpression = `(?m:\n\s{6}annotations:\s*(?:#.*)*(?:\n\s{8}.*)*$)`
		// Grab the entire metadata block.
		// We need to know the name of the resource to decide whether or not to update its annotations
		metadataExpression = `(?m:\n\s{4}kind:(?:.*)\n\s{4}metadata:(?:.*)*\s*(?:#.*)*(?:\n\s{6}.*)*$)`
	} else {
		annotationsExpression = `(?m:\n  annotations:\s*(?:#.*)*(?:\n    .*)*$)`
		metadataExpression = `(?m:^(metadata:\s*(?:#.*)*)$)`
		annotations = manifest.Metadata.AnnotationsOrNil()
		containers = manifest.Spec.Template.Spec.Containers
	}

	if tagAll != "" {
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
		fragment = indent(fragment)

		// Add a newline if it's not blank
		if len(fragment) > 0 {
			fragment = "\n" + fragment
		}
	}

	// Find where to insert the fragment.
	// TODO: This should handle potentially different indentation.
	// TODO: There's probably a more elegant regex-ey way to do this in one pass.
	replaced := false
	annotationsRE := regexp.MustCompile(annotationsExpression)

	var newDef string
	if !isList {
		// Don't try to handle List stuff here. The metadataRE will take care of it.
		newDef = annotationsRE.ReplaceAllStringFunc(str, func(found string) string {
			if !replaced {
				replaced = true
				return fragment
			}
			return found
		})
	}

	if !replaced {
		metadataRE := regexp.MustCompile(metadataExpression)
		newDef = metadataRE.ReplaceAllStringFunc(str, func(found string) string {
			// `found` contains the entire metadata block.
			switch {
			case replaced:
				return found
			case !isList:
				replaced = true
				return found + fragment
			case shouldUpdateAnnotations(found, serviceID):
				// Doing List stuff here
				// The metadata must contain the right serviceID in order to replace the annotations
				// Find and replace only the annotations block within the metadata block
				// List item annotations have a little more indentation than regular files
				indented := indent(indent(fragment))
				f := found + indented
				// If annotations already exist, replace them.
				// If not, just return the metadata block with the new annotations
				if annotationsRE.MatchString(found) {
					f = annotationsRE.ReplaceAllString(found, indented)
				}
				replaced = true
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

func parseManifest(def []byte) (resource.BaseObject, error) {
	var m resource.BaseObject
	if err := yaml.Unmarshal(def, &m); err != nil {
		return m, errors.Wrap(err, "decoding annotations")
	}
	return m, nil
}

func (m *Manifests) ServicesWithPolicies(root string) (policy.ResourceMap, error) {
	manifests, err := m.LoadManifests(root)

	if err != nil {
		return nil, err
	}

	result := map[flux.ResourceID]policy.Set{}
	for name, r := range manifests {
		resourceID, err := flux.ParseResourceID(name)
		if err != nil {
			return nil, err
		}
		result[resourceID] = r.Policy()
	}

	return result, nil
}

func indent(str string) string {
	return regexp.MustCompile(`(.+)`).ReplaceAllString(str, "  $1")
}

func contains(target, substring string) bool {
	return strings.Contains(strings.ToLower(target), substring)
}

func shouldUpdateAnnotations(found string, r flux.ResourceID) bool {
	// Avoid updating resources with the same name, ie a Deployment named 'foo' AND a Service named 'foo'
	namespace, kind, name := r.Components()
	if namespace == "default" {
		// No namespace specified, only match Kind and name
		return contains(found, "kind: "+kind) && contains(found, "name: "+name)
	}
	// Avoid updating two deployments with the same name in different namespaces (defined in the same List)
	return contains(found, "namespace: "+namespace) && contains(found, "kind: "+kind) && contains(found, "name: "+name)
}
