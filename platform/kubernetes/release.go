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
)

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

func (c *Cluster) doCommand(logger log.Logger, newDefinition []byte, args ...string) error {
	cmd := c.kubectlCommand(args...)
	cmd.Stdin = bytes.NewReader(newDefinition)
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

// Sadly, replication controllers can't be `kubectl apply`ed like
// deployments can. As a hack, we just start the process off in a
// goroutine; it'll get logged, at least, even if we don't get the
// result.
func (c *Cluster) startRollingUpgrade(logger log.Logger, newDef *apiObject) error {
	go func() {
		c.doCommand(
			logger,
			newDef.bytes,
			"rolling-update",
			"--update-period", "3s",
			"--namespace", newDef.Metadata.Namespace,
			newDef.Metadata.Name,
			"-f", "-", // take definition from stdin
		)
	}()
	return nil
}
