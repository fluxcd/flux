package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	rest "k8s.io/client-go/rest"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
)

type changeSet struct {
	objs map[string][]*apiObject
}

func makeChangeSet() changeSet {
	return changeSet{objs: make(map[string][]*apiObject)}
}

func (c *changeSet) stage(cmd string, o *apiObject) {
	c.objs[cmd] = append(c.objs[cmd], o)
}

// Applier is something that will apply a changeset to the cluster.
type Applier interface {
	apply(log.Logger, changeSet, map[flux.ResourceID]error) cluster.SyncError
}

type Kubectl struct {
	exe    string
	config *rest.Config
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

// rankOfKind returns an int denoting the position of the given kind
// in the partial ordering of Kubernetes resources, according to which
// kinds depend on which (derived by hand).
func rankOfKind(kind string) int {
	switch kind {
	// Namespaces answer to NOONE
	case "Namespace":
		return 0
	// These don't go in namespaces; or do, but don't depend on anything else
	case "CustomResourceDefinition", "ServiceAccount", "ClusterRole", "Role", "PersistentVolume", "Service":
		return 1
	// These depend on something above, but not each other
	case "ResourceQuota", "LimitRange", "Secret", "ConfigMap", "RoleBinding", "ClusterRoleBinding", "PersistentVolumeClaim", "Ingress":
		return 2
	// Same deal, next layer
	case "DaemonSet", "Deployment", "ReplicationController", "ReplicaSet", "Job", "CronJob", "StatefulSet":
		return 3
	// Assumption: anything not mentioned isn't depended _upon_, so
	// can come last.
	default:
		return 4
	}
}

type applyOrder []*apiObject

func (objs applyOrder) Len() int {
	return len(objs)
}

func (objs applyOrder) Swap(i, j int) {
	objs[i], objs[j] = objs[j], objs[i]
}

func (objs applyOrder) Less(i, j int) bool {
	ranki, rankj := rankOfKind(objs[i].Kind), rankOfKind(objs[j].Kind)
	if ranki == rankj {
		return objs[i].Metadata.Name < objs[j].Metadata.Name
	}
	return ranki < rankj
}

func (c *Kubectl) apply(logger log.Logger, cs changeSet, errored map[flux.ResourceID]error) (errs cluster.SyncError) {
	f := func(objs []*apiObject, cmd string, args ...string) {
		if len(objs) == 0 {
			return
		}
		logger.Log("cmd", cmd, "args", strings.Join(args, " "), "count", len(objs))
		args = append(args, cmd)

		var multi, single []*apiObject
		if len(errored) == 0 {
			multi = objs
		} else {
			for _, obj := range objs {
				if _, ok := errored[obj.ResourceID()]; ok {
					// Resources that errored before shall be applied separately
					single = append(single, obj)
				} else {
					// everything else will be tried in a multidoc apply.
					multi = append(multi, obj)
				}
			}
		}

		if len(multi) > 0 {
			if err := c.doCommand(logger, makeMultidoc(multi), args...); err != nil {
				single = append(single, multi...)
			}
		}
		for _, obj := range single {
			r := bytes.NewReader(obj.Bytes())
			if err := c.doCommand(logger, r, args...); err != nil {
				errs = append(errs, cluster.ResourceError{obj.Resource, err})
			}
		}
	}

	// When deleting objects, the only real concern is that we don't
	// try to delete things that have already been deleted by
	// Kubernete's GC -- most notably, resources in a namespace which
	// is also being deleted. GC does not have the dependency ranking,
	// but we can use it as a shortcut to avoid the above problem at
	// least.
	objs := cs.objs["delete"]
	sort.Sort(sort.Reverse(applyOrder(objs)))
	f(objs, "delete")

	objs = cs.objs["apply"]
	sort.Sort(applyOrder(objs))
	f(objs, "apply")
	return errs
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

func makeMultidoc(objs []*apiObject) *bytes.Buffer {
	buf := &bytes.Buffer{}
	for _, obj := range objs {
		buf.WriteString("\n---\n")
		buf.Write(obj.Bytes())
	}
	return buf
}

func (c *Kubectl) kubectlCommand(args ...string) *exec.Cmd {
	return exec.Command(c.exe, append(c.connectArgs(), args...)...)
}
