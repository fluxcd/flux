package kubernetes

import (
	"fmt"
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
	fmt.Println("\t\t>>> FindDefinedServices")

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
	fmt.Printf("\t\t!!! FindDefinedServices: %v\n", result)

	return result, nil
}
