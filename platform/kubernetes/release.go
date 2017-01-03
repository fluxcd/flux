package kubernetes

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"k8s.io/kubernetes/pkg/api"
	apiext "k8s.io/kubernetes/pkg/apis/extensions"

	"github.com/weaveworks/flux/platform"
)

func (c podController) newRegrade(newDefinition *apiObject) (*regrade, error) {
	k := c.kind()
	if newDefinition.Kind != k {
		return nil, fmt.Errorf(`Expected new definition of kind %q, to match old definition; got %q`, k, newDefinition.Kind)
	}

	var result regrade
	switch {
	case c.Deployment != nil:
		result.exec = deploymentExec(c.Deployment, newDefinition)
		result.summary = "Applying deployment"
	case c.ReplicationController != nil:
		result.exec = rollingUpgradeExec(c.ReplicationController, newDefinition)
		result.summary = "Rolling upgrade"
	case c.DaemonSet != nil:
		result.exec = daemonsetExec(c.DaemonSet, newDefinition)
		result.summary = "Applying DaemonSet"
	default:
		return nil, platform.ErrNoMatching
	}
	return &result, nil
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
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	logger.Log("cmd", strings.Join(args, " "))

	begin := time.Now()
	err := cmd.Run()
	result := "success"
	if err != nil {
		result = stderr.String()
		err = errors.Wrap(errors.New(result), "running kubectl")
	}
	logger.Log("result", result, "took", time.Since(begin).String())
	return err
}

func rollingUpgradeExec(def *api.ReplicationController, newDef *apiObject) regradeExecFunc {
	return func(c *Cluster, logger log.Logger) error {
		return c.doReleaseCommand(
			logger,
			newDef,
			"rolling-update",
			"--update-period", "3s",
			def.Name,
			"-f", "-", // take definition from stdin
		)
	}
}

func deploymentExec(def *apiext.Deployment, newDef *apiObject) regradeExecFunc {
	return func(c *Cluster, logger log.Logger) error {
		err := c.doReleaseCommand(
			logger,
			newDef,
			"apply",
			"-f", "-", // take definition from stdin
		)

		if err == nil {
			args := []string{
				"rollout", "status",
				"deployment", newDef.Metadata.Name,
				"--namespace", newDef.Metadata.Namespace,
			}
			cmd := c.kubectlCommand(args...)
			logger.Log("cmd", strings.Join(args, " "))
			err = cmd.Run()
		}
		return err
	}
}

func daemonsetExec(def *apiext.DaemonSet, newDef *apiObject) regradeExecFunc {
	return func(c *Cluster, logger log.Logger) error {
		return c.doReleaseCommand(
			logger,
			newDef,
			"apply",
			"-f", "-", // take definition from stdin
		)
	}
}
