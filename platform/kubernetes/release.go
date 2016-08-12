package kubernetes

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"k8s.io/kubernetes/pkg/api"
	apiext "k8s.io/kubernetes/pkg/apis/extensions"

	"github.com/weaveworks/fluxy/platform"
)

func (c podController) createPlan(newDefinition *apiObject) (releasePlan, error) {
	k := c.kind()
	if newDefinition.Kind != k {
		return nil, fmt.Errorf(`Expected new definition of kind %q, to match old definition; got %q`, k, newDefinition.Kind)
	}

	if c.Deployment != nil {
		return &releaseDeployment{c.Deployment, newDefinition}, nil
	} else if c.ReplicationController != nil {
		return &releaseReplicationController{c.ReplicationController, newDefinition}, nil
	} else {
		return nil, platform.ErrNoMatching
	}
}

type releaseReplicationController struct {
	rc            *api.ReplicationController
	newDefinition *apiObject
}

func (c *Cluster) connectArgs() []string {
	var args []string
	if c.config.Host != "" {
		args = append(args, fmt.Sprintf("--server=%s", c.config.Host))
	}
	if c.config.Username != "" {
		args = append(args, fmt.Sprintf("--username=%s", c.config.Username))
	}
	if c.config.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", c.config.Password))
	}
	if c.config.TLSClientConfig.CertFile != "" {
		args = append(args, fmt.Sprintf("--client-certificate=%s", c.config.TLSClientConfig.CertFile))
	}
	if c.config.TLSClientConfig.CAFile != "" {
		args = append(args, fmt.Sprintf("--certificate-authority=%s", c.config.TLSClientConfig.CAFile))
	}
	if c.config.TLSClientConfig.KeyFile != "" {
		args = append(args, fmt.Sprintf("--client-key=%s", c.config.TLSClientConfig.KeyFile))
	}
	if c.config.BearerToken != "" {
		args = append(args, fmt.Sprintf("--token=%s", c.config.BearerToken))
	}
	return args
}

func (c *Cluster) kubectlCommand(args ...string) *exec.Cmd {
	cmd := exec.Command(c.kubectl, append(c.connectArgs(), args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (c *Cluster) doReleaseCommand(logger log.Logger, newDefinition *apiObject, args ...string) error {
	cmd := c.kubectlCommand(args...)
	cmd.Stdin = bytes.NewReader(newDefinition.bytes)
	logger.Log("cmd", strings.Join(cmd.Args, " "))

	begin := time.Now()
	err := cmd.Run()
	result := "success"
	if err != nil {
		result = err.Error()
	}
	logger.Log("result", result, "took", time.Since(begin).String())
	return err
}

func (r *releaseReplicationController) do(c *Cluster, logger log.Logger) error {
	return c.doReleaseCommand(
		logger,
		r.newDefinition,
		"rolling-update",
		"--update-period", "3s",
		r.rc.Name,
		"-f", "-", // take definition from stdin
	)
}

func (r *releaseReplicationController) summary() string {
	return "Rolling upgrade in progress"
}

type releaseDeployment struct {
	deployment    *apiext.Deployment
	newDefinition *apiObject
}

func (r *releaseDeployment) do(c *Cluster, logger log.Logger) error {
	err := c.doReleaseCommand(
		logger,
		r.newDefinition,
		"apply",
		"-f", "-", // take definition from stdin
	)

	if err == nil {
		cmd := c.kubectlCommand(
			"rollout", "status",
			"deployment", r.newDefinition.Metadata.Name,
		)
		logger.Log("cmd", strings.Join(cmd.Args, " "))
		err = cmd.Run()
	}
	return err
}

func (r *releaseDeployment) summary() string {
	return "Deployment rollout in progress"
}
