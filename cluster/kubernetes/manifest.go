package kubernetes

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/weaveworks/flux"
)

// UpdateManifest looks for the manifest for a given service, reads its
// contents, applies f(contents), and writes the results back to the file.
// TODO: It is super inefficient, as it calls kubeservice on all files every
// time.
func (c *Cluster) UpdateManifest(root string, serviceID string, f func(manifest []byte) ([]byte, error)) error {
	services, err := c.FindDefinedServices(root)
	if err != nil {
		return err
	}
	paths := services[flux.ServiceID(serviceID)]
	if len(paths) == 0 {
		return fmt.Errorf("no resource file found for service %s", serviceID)
	}
	if len(paths) > 1 {
		return fmt.Errorf("multiple resource files found for service %s: %s", serviceID, strings.Join(paths, ", "))
	}

	def, err := ioutil.ReadFile(paths[0])
	if err != nil {
		return err
	}

	newDef, err := f(def)
	if err != nil {
		return err
	}

	fi, err := os.Stat(paths[0])
	if err != nil {
		return err
	}
	return ioutil.WriteFile(paths[0], newDef, fi.Mode())
}
