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
	api "k8s.io/client-go/1.5/pkg/api/v1"
	apiext "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"

	"github.com/weaveworks/flux/platform"
)

func (c podController) newApply(newDefinition *apiObject, async bool) (*apply, error) {
	k := c.kind()
	if newDefinition.Kind != k {
		return nil, fmt.Errorf(`Expected new definition of kind %q, to match old definition; got %q`, k, newDefinition.Kind)
	}

	var result apply
	if c.Deployment != nil {
		result.exec = deploymentExec(c.Deployment, newDefinition, async)
		result.summary = "Applying deployment"
	} else if c.ReplicationController != nil {
		result.exec = rollingUpgradeExec(c.ReplicationController, newDefinition, async)
		result.summary = "Rolling upgrade"
	} else {
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

func (c *Cluster) doApplyCommand(logger log.Logger, newDefinition *apiObject, args ...string) error {
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

func rollingUpgradeExec(def *api.ReplicationController, newDef *apiObject, async bool) applyExecFunc {
	return func(c *Cluster, logger log.Logger) error {
		err := make(chan error)
		go func() {
			err <- c.doApplyCommand(
				logger,
				newDef,
				"rolling-update",
				"--update-period", "3s",
				def.Name,
				"-f", "-", // take definition from stdin
			)
		}()
		if async {
			return nil
		}
		return <-err
	}
}

func deploymentExec(def *apiext.Deployment, newDef *apiObject, async bool) applyExecFunc {
	return func(c *Cluster, logger log.Logger) error {
		err := c.doApplyCommand(
			logger,
			newDef,
			"apply",
			"-f", "-", // take definition from stdin
		)
		if async {
			return err
		}

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
