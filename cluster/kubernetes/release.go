package kubernetes

import (
	"bytes"
	"fmt"
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

	*changeSet
}

func NewKubectl(exe string, config *rest.Config) *Kubectl {
	return &Kubectl{
		exe:       exe,
		config:    config,
		changeSet: newChangeSet(),
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
	for id, obj := range c.deleteObjs {
		logger := log.With(logger, "resource", id)
		if err := c.delete(logger, obj.bytes); err != nil {
			errs[id] = err
		}
	}
	for id, obj := range c.applyObjs {
		logger := log.With(logger, "resource", id)
		if err := c.apply(logger, obj.bytes); err != nil {
			errs[id] = err
		}
	}
	c.changeSet = newChangeSet()
	if len(errs) != 0 {
		return errs
	}
	return nil
}

func (c *Kubectl) delete(logger log.Logger, b []byte) error {
	return c.doCommand(logger, b, "delete", "-f", "-")
}

func (c *Kubectl) apply(logger log.Logger, b []byte) error {
	return c.doCommand(logger, b, "apply", "-f", "-")
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

func (c *Kubectl) kubectlCommand(args ...string) *exec.Cmd {
	return exec.Command(c.exe, append(c.connectArgs(), args...)...)
}

type changeSet struct {
	deleteObjs, applyObjs map[string]*apiObject
}

func newChangeSet() *changeSet {
	return &changeSet{
		deleteObjs: make(map[string]*apiObject),
		applyObjs:  make(map[string]*apiObject),
	}
}
func (c *changeSet) stageDelete(id string, obj *apiObject) {
	c.deleteObjs[id] = obj
}

func (c *changeSet) stageApply(id string, obj *apiObject) {
	c.applyObjs[id] = obj
}
