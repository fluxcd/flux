package docker

import (
	"bytes"
	"io"
	"os/exec"
	"strings"

	"github.com/weaveworks/flux"
	dresource "github.com/weaveworks/flux/cluster/docker/resource"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
	yaml "gopkg.in/yaml.v2"
)

type Manifests struct {
	Namespace string
}

// FindDefinedServices implementation in files.go

func (c *Manifests) LoadManifests(paths ...string) (map[string]resource.Resource, error) {
	return dresource.Load(c.Namespace, paths...)
}

func (c *Manifests) ParseManifests(allDefs []byte) (map[string]resource.Resource, error) {
	return dresource.ParseMultidoc(allDefs, "exported")
}

func (c *Manifests) UpdateDefinition(def []byte, container string, image flux.ImageID) ([]byte, error) {
	var mc minimalCompose

	err := yaml.Unmarshal(def, &mc)
	if err != nil {
		return nil, err
	}

	for _, v := range mc.Services {
		m := v.(map[interface{}]interface{})
		m["image"] = image.FullID()
	}

	ret, err := yaml.Marshal(mc)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = c.tryUpdate(ret, container, image, &buf)

	return ret, err
}

func (c *Manifests) tryUpdate(def []byte, container string, newImage flux.ImageID, out io.Writer) error {
	stderr := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	docker, err := exec.LookPath("docker")
	svc := strings.Split(container, ".")[0]

	cmd := exec.Command(docker, "service", "update", "--image", newImage.FullID(), svc)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	err = cmd.Run()

	return err

}

func (c *Manifests) UpdatePolicies([]byte, policy.Update) ([]byte, error) {
	return nil, nil
}

func (c *Manifests) ServicesWithPolicies(path string) (policy.ServiceMap, error) {
	return policy.ServiceMap{}, nil
}
