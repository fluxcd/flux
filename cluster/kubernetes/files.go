package kubernetes

import (
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes/resource"
)

// FindDefinedServices finds all the services defined under the
// directory given, and returns a map of service IDs (from its
// specified namespace and name) to the paths of resource definition
// files.
func (c *Manifests) FindDefinedServices(path string) (map[flux.ResourceID][]string, error) {
	objects, err := resource.Load(path, path)
	if err != nil {
		return nil, errors.Wrap(err, "loading resources")
	}

	var result = map[flux.ResourceID][]string{}
	for _, obj := range objects {
		id := obj.ResourceID()
		_, kind, _ := id.Components()
		if _, ok := resourceKinds[kind]; ok {
			result[id] = append(result[id], filepath.Join(path, obj.Source()))
		}
	}
	return result, nil
}

// FindNamespaces finds all the namespaces defined under the directory
// given, and returns a map of namespace IDs (default:namespace:<namespace>)
// to the paths of the namespace defintion files.
func (c *Manifests) FindNamespaces(path string) (map[flux.ResourceID][]string, error) {
	objects, err := resource.Load(path, path)
	if err != nil {
		return nil, errors.Wrap(err, "loading resources")
	}

	var result = map[flux.ResourceID][]string{}
	for _, obj := range objects {
		id := obj.ResourceID()
		_, kind, _ := id.Components()
		if kind == "namespace" {
			result[id] = append(result[id], filepath.Join(path, obj.Source()))
		}
	}
	return result, nil
}
