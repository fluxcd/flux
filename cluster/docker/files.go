package docker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
)

type minimalCompose struct {
	Version  string                 `yaml:"version"`
	Services map[string]interface{} `yaml:"services"`
}

// FindDefinedServices finds all the services defined under the
// directory given, and returns a map of service IDs (from its
// specified namespace and name) to the paths of resource definition
// files.
func (c *Manifests) FindDefinedServices(path string) (map[flux.ServiceID][]string, error) {
	var files []string
	filepath.Walk(path, func(target string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if ext := filepath.Ext(target); ext == ".yaml" || ext == ".yml" {
			files = append(files, target)
		}
		return nil
	})

	services := map[flux.ServiceID][]string{}
	for _, file := range files {
		fc, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}
		var def minimalCompose
		err = yaml.Unmarshal(fc, &def)
		if err != nil {
			continue
		}

		if len(def.Services) > 1 {
			return services, fmt.Errorf("Expected one service per yaml found %v in %v", len(def.Services), file)
		}
		for k, _ := range def.Services {
			id := flux.MakeServiceID(c.Namespace, k)
			services[id] = append(services[id], file)
		}
	}
	return services, nil
}
