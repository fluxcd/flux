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
	"github.com/weaveworks/flux/cluster"
	rest "k8s.io/client-go/rest"
)

type Kubectl struct {
	exe    string
	config *rest.Config

	changeSet
}

func NewKubectl(exe string, config *rest.Config) *Kubectl {
	return &Kubectl{
		exe:    exe,
		config: config,
	}
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

func (c *Kubectl) execute(logger log.Logger, errs cluster.SyncError) error {
	defer c.changeSet.clear()

	deleteBuf := &bytes.Buffer{}
	for _, obj := range c.deleteObjs {
		fmt.Fprintln(deleteBuf, "---")
		fmt.Fprintln(deleteBuf, string(obj.bytes))
	}

	if err := c.doCommand(logger, "delete", deleteBuf); err != nil {
		errs["deleting"] = err
	}

	applyBuf := &bytes.Buffer{}
	for _, obj := range c.applyObjs {
		fmt.Fprintln(applyBuf, "---")
		fmt.Fprintln(applyBuf, string(obj.bytes))
	}

	if err := c.doCommand(logger, "apply", applyBuf); err != nil {
		errs["applying"] = err
	}

	if len(errs) != 0 {
		return errs
	}
	return nil
}

func (c *Kubectl) doCommand(logger log.Logger, command string, r io.Reader) error {
	args := []string{command, "-f", "-"}
	cmd := c.kubectlCommand(args...)
	cmd.Stdin = r
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

func (c *Kubectl) kubectlCommand(args ...string) *exec.Cmd {
	return exec.Command(c.exe, append(c.connectArgs(), args...)...)
}

type changeSet struct {
	deleteObjs, applyObjs []*apiObject
}

func (c *changeSet) stageDelete(obj *apiObject) {
	c.deleteObjs = append(c.deleteObjs, obj)
}

func (c *changeSet) stageApply(obj *apiObject) {
	c.applyObjs = append(c.applyObjs, obj)
}

func (c *changeSet) clear() {
	c.deleteObjs = nil
	c.applyObjs = nil
}
