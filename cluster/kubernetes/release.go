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

var (
	cmds = []string{"delete", "apply"}
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

func (c *Kubectl) execute(logger log.Logger, errs cluster.SyncError) {
	defer c.changeSet.clear()

	for _, cmd := range cmds {
		buf := &bytes.Buffer{}
		for _, obj := range c.objs[cmd] {
			fmt.Fprintln(buf, "---")
			fmt.Fprintln(buf, string(obj.bytes))
		}

		if err := c.doCommand(logger, cmd, buf); err != nil {
			for _, obj := range c.objs[cmd] {
				r := bytes.NewReader(obj.bytes)
				if err := c.doCommand(logger, cmd, r); err != nil {
					errs[obj.Metadata.Name] = err
				}
			}
		}
	}
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
	objs map[string][]obj
}

func (c *changeSet) stage(cmd, id string, o *apiObject) {
	if c.objs == nil {
		c.objs = make(map[string][]obj)
	}
	c.objs[cmd] = append(c.objs[cmd], obj{id, o})
}

func (c *changeSet) clear() {
	if c.objs == nil {
		c.objs = make(map[string][]obj)
		return
	}
	for cmd := range c.objs {
		c.objs[cmd] = nil
	}
}

type obj struct {
	id string
	*apiObject
}
