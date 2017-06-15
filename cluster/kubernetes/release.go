package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	rest "k8s.io/client-go/1.5/rest"
)

func NewKubectl(exe string, config *rest.Config, stdout, stderr io.Writer) *Kubectl {
	return &Kubectl{exe, config, stdout, stderr}
}

type Kubectl struct {
	exe            string
	config         *rest.Config
	stdout, stderr io.Writer
}

func (c *Kubectl) connectArgs() []string {
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

func (c *Kubectl) kubectlCommand(args ...string) *exec.Cmd {
	cmd := exec.Command(c.exe, append(c.connectArgs(), args...)...)
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr
	return cmd
}

func (c *Kubectl) doCommand(logger log.Logger, newDefinition []byte, args ...string) error {
	cmd := c.kubectlCommand(args...)
	cmd.Stdin = bytes.NewReader(newDefinition)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout

	begin := time.Now()
	err := cmd.Run()
	if err != nil {
		err = errors.Wrap(errors.New(strings.TrimSpace(stderr.String())), "running kubectl")
	}

	logger.Log("cmd", "kubectl "+strings.Join(args, " "), "took", time.Since(begin), "err", err, "output", strings.TrimSpace(stdout.String()))
	return err
}

func (c *Kubectl) Delete(logger log.Logger, obj *apiObject) error {
	return c.doCommand(logger, obj.bytes, "--namespace", obj.namespaceOrDefault(), "delete", "-f", "-")
}

func (c *Kubectl) Apply(logger log.Logger, obj *apiObject) error {
	return c.doCommand(logger, obj.bytes, "--namespace", obj.namespaceOrDefault(), "apply", "-f", "-")
}
