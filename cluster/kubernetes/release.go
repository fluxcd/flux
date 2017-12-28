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

func (c *Kubectl) execute(logger log.Logger, errs cluster.SyncError) {
	defer c.changeSet.clear()

	f := func(m map[string][]obj, cmd string, args ...string) {
		objs := m[cmd]
		if len(objs) == 0 {
			return
		}
		args = append(args, cmd)
		if err := c.doCommand(logger, makeMultidoc(objs), args...); err != nil {
			for _, obj := range objs {
				r := bytes.NewReader(obj.bytes)
				if err := c.doCommand(logger, r, args...); err != nil {
					errs[obj.id] = err
				}
			}
		}
	}

	// When deleting resources we must ensure any resource in a non-default
	// namespace is deleted before the namespace that it is in. Since namespace
	// resources don't specify a namespace, this ordering guarantees that.
	f(c.nsObjs, "delete")
	f(c.noNsObjs, "delete", "--namespace", "default")
	// Likewise, when applying resources we must ensure the namespace is applied
	// first, so we run the commands the other way round.
	f(c.noNsObjs, "apply", "--namespace", "default")
	f(c.nsObjs, "apply")

}

func (c *Kubectl) doCommand(logger log.Logger, r io.Reader, args ...string) error {
	args = append(args, "-f", "-")
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

func makeMultidoc(objs []obj) *bytes.Buffer {
	buf := &bytes.Buffer{}
	for _, obj := range objs {
		buf.WriteString("---\n" + string(obj.bytes))
	}
	return buf
}

func (c *Kubectl) kubectlCommand(args ...string) *exec.Cmd {
	return exec.Command(c.exe, append(c.connectArgs(), args...)...)
}

type changeSet struct {
	nsObjs   map[string][]obj
	noNsObjs map[string][]obj
}

func (c *changeSet) init() {
	if c.nsObjs == nil {
		c.nsObjs = make(map[string][]obj)
	}
	if c.noNsObjs == nil {
		c.noNsObjs = make(map[string][]obj)
	}
}

func (c *changeSet) stage(cmd, id string, o *apiObject) {
	c.init()
	if o.hasNamespace() {
		c.nsObjs[cmd] = append(c.nsObjs[cmd], obj{id, o})
	} else {
		c.noNsObjs[cmd] = append(c.noNsObjs[cmd], obj{id, o})
	}
}

func (c *changeSet) clear() {
	c.nsObjs = nil
	c.noNsObjs = nil
	c.init()
}

type obj struct {
	id string
	*apiObject
}
