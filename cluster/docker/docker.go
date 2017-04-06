package docker

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/client"
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/ssh"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

type Swarm struct {
	client    *client.Client
	actionc   chan func()
	namespace string
	logger    log.Logger
}

type apiObject struct {
	bytes    []byte
	Version  string                            `yaml:"version"`
	Services map[string]map[string]interface{} `yaml:"services"`
}

func NewSwarm(namespace string, logger log.Logger) (*Swarm, error) {
	cli, err := client.NewEnvClient()

	if err != nil {
		return nil, err
	}

	c := &Swarm{
		client:    cli,
		actionc:   make(chan func()),
		namespace: namespace,
		logger:    logger,
	}

	go c.loop()
	return c, nil
}

// Stop terminates the goroutine that serializes and executes requests against
// the cluster. A stopped cluster cannot be restarted.
func (c *Swarm) Stop() {
	close(c.actionc)
}

func (c *Swarm) loop() {
	for f := range c.actionc {
		f()
	}
}

func (c *Swarm) PublicSSHKey(regenerate bool) (ssh.PublicKey, error) {
	return ssh.PublicKey{}, nil
}

func (c *Swarm) removeService(logger log.Logger, obj *apiObject) error {
	var service string

	stderr := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	docker, err := exec.LookPath("docker")

	if err != nil {
		logger.Log(err)
	}
	for r, _ := range obj.Services {
		service = fmt.Sprintf("%s_%s", c.namespace, r)
	}

	cmd := exec.Command(docker, "service", "rm", service)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	err = cmd.Run()

	return err
}

func (c *Swarm) applyService(logger log.Logger, obj *apiObject) error {
	var image string
	var service string

	stderr := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	docker, err := exec.LookPath("docker")

	for r, v := range obj.Services {
		for k, x := range v {
			if k == "image" {
				service = fmt.Sprintf("%s_%s", c.namespace, r)
				image = x.(string)
				break
			}
		}
	}

	svcs, err := c.AllServices(c.namespace)

	var exists bool
	var cmd *exec.Cmd

	for _, svc := range svcs {
		k := strings.Replace(svc.ID.String(), "/", "_", 1)
		if k == service {
			exists = true
		}
	}

	if exists {
		cmd = exec.Command(docker, "service", "update", "--image", image, service)
	} else {
		tmpfile, err := ioutil.TempFile("", "temp")
		if err != nil {
			c.logger.Log(err)
		}

		defer os.Remove(tmpfile.Name()) //clean up

		if _, err := tmpfile.Write(obj.bytes); err != nil {
			c.logger.Log(err)
		}
		if err := tmpfile.Close(); err != nil {
			c.logger.Log(err)
		}

		cmd = exec.Command(docker, "deploy", "-c", tmpfile.Name(), c.namespace)
	}
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	err = cmd.Run()

	return err
}

func (c *Swarm) Sync(spec cluster.SyncDef) error {
	errc := make(chan error)
	logger := log.NewContext(c.logger).With("method", "Sync")
	c.actionc <- func() {
		errs := cluster.SyncError{}
		for _, action := range spec.Actions {
			logger := log.NewContext(logger).With("resource", action.ResourceID)
			if len(action.Delete) > 0 {
				obj, err := definitionObj(action.Delete)
				if err == nil {
					err = c.removeService(logger, obj)
				}
				if err != nil {
					errs[action.ResourceID] = err
					continue
				}
			}
			if len(action.Apply) > 0 {
				obj, err := definitionObj(action.Apply)
				if err == nil {
					err = c.applyService(logger, obj)
				}
				if err != nil {
					errs[action.ResourceID] = err
					continue
				}
			}
		}
		if len(errs) > 0 {
			errc <- errs
		} else {
			errc <- nil
		}
	}
	return <-errc
}

func (c *Swarm) Ping() error {
	ctx := context.Background()
	_, err := c.client.ServerVersion(ctx)
	return err
}

func (c *Swarm) Export() ([]byte, error) {
	var config bytes.Buffer
	config.WriteString("version: '3'\n")
	config.WriteString("\n")
	config.WriteString("services:\n")
	return config.Bytes(), nil
}

func (c *Swarm) ImagesToFetch() (imageCreds registry.ImageCreds) {
	imageCreds = make(registry.ImageCreds, 0)
	svcs, err := c.AllServices("")

	if err != nil {
		c.logger.Log("err", err)
	}

	for _, svc := range svcs {
		if len(svc.Containers.Containers) >= 1 {
			container := svc.Containers.Containers[0]
			r, err := flux.ParseImageID(container.Image)
			if err != nil {
				c.logger.Log("err", err)

			}
			imageCreds[r] = registry.NoCredentials()
		}
	}

	return
}

// A convenience for getting an minimal object from some bytes.
func definitionObj(bytes []byte) (*apiObject, error) {
	obj := apiObject{bytes: bytes}
	return &obj, yaml.Unmarshal(bytes, &obj)
}
