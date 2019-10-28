package kubernetes

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"github.com/fluxcd/flux/pkg/cluster"
	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
)

const (
	// We use mark-and-sweep garbage collection to delete cluster objects.
	// Marking is done by adding a label when creating and updating the objects.
	// Sweeping is done by comparing Marked cluster objects with the manifests in Git.
	gcMarkLabel = kresource.PolicyPrefix + "sync-gc-mark"
	// We want to prevent garbage-collecting cluster objects which haven't been updated.
	// We annotate objects with the checksum of their Git manifest to verify this.
	checksumAnnotation = kresource.PolicyPrefix + "sync-checksum"
)

// --- internals in support of Sync

type kuberesource struct {
	obj        *unstructured.Unstructured
	namespaced bool
}

// ResourceID returns the ID for this resource loaded from the
// cluster.
func (r *kuberesource) ResourceID() resource.ID {
	ns, kind, name := r.obj.GetNamespace(), r.obj.GetKind(), r.obj.GetName()
	if !r.namespaced {
		ns = kresource.ClusterScope
	}
	return resource.MakeID(ns, kind, name)
}

// Bytes returns a byte slice description, including enough info to
// identify the resource (but not momre)
func (r *kuberesource) IdentifyingBytes() []byte {
	return []byte(fmt.Sprintf(`
apiVersion: %s
kind: %s
metadata:
  namespace: %q
  name: %q
`, r.obj.GetAPIVersion(), r.obj.GetKind(), r.obj.GetNamespace(), r.obj.GetName()))
}

func (r *kuberesource) Policies() policy.Set {
	return kresource.PoliciesFromAnnotations(r.obj.GetAnnotations())
}

func (r *kuberesource) GetChecksum() string {
	return r.obj.GetAnnotations()[checksumAnnotation]
}

func (r *kuberesource) GetGCMark() string {
	return r.obj.GetLabels()[gcMarkLabel]
}

func ApplyMetadata(res resource.Resource, syncSetName, checksum string, mixinLabels map[string]string) ([]byte, error) {
	definition := map[interface{}]interface{}{}
	if err := yaml.Unmarshal(res.Bytes(), &definition); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to parse yaml from %s", res.Source()))
	}

	mixin := map[string]interface{}{}

	if syncSetName != "" {
		if mixinLabels == nil {
			mixinLabels = map[string]string{}
		}
		mixinLabels[gcMarkLabel] = makeGCMark(syncSetName, res.ResourceID().String())
		mixin["labels"] = mixinLabels
	}

	if checksum != "" {
		mixinAnnotations := map[string]string{}
		mixinAnnotations[checksumAnnotation] = checksum
		mixin["annotations"] = mixinAnnotations
	}

	mergo.Merge(&definition, map[interface{}]interface{}{
		"metadata": mixin,
	})

	bytes, err := yaml.Marshal(definition)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize yaml after applying metadata")
	}
	return bytes, nil
}

func AllowedForGC(obj *unstructured.Unstructured, syncSetName string) bool {
	res := kuberesource{obj: obj, namespaced: obj.GetNamespace() != ""}
	return res.GetGCMark() == makeGCMark(syncSetName, res.ResourceID().String())
}

func makeGCMark(syncSetName, resourceID string) string {
	hasher := sha256.New()
	hasher.Write([]byte(syncSetName))
	// To prevent deleting objects with copied labels
	// an object-specific mark is created (by including its identifier).
	hasher.Write([]byte(resourceID))
	// The prefix is to make sure it's a valid (Kubernetes) label value.
	return "sha256." + base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))
}

// --- internal types for keeping track of syncing

type applyObject struct {
	ResourceID resource.ID
	Source     string
	Payload    []byte
}

type changeSet struct {
	objs map[string][]applyObject
}

func makeChangeSet() changeSet {
	return changeSet{objs: make(map[string][]applyObject)}
}

func (c *changeSet) stage(cmd string, id resource.ID, source string, bytes []byte) {
	c.objs[cmd] = append(c.objs[cmd], applyObject{id, source, bytes})
}

// Applier is something that will apply a changeset to the cluster.
type Applier interface {
	apply(log.Logger, changeSet, map[resource.ID]error) cluster.SyncError
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
	switch strings.ToLower(kind) {
	// Namespaces answer to NOONE
	case "namespace":
		return 0
	// These don't go in namespaces; or do, but don't depend on anything else
	case "customresourcedefinition", "serviceaccount", "clusterrole", "role", "persistentvolume", "service":
		return 1
	// These depend on something above, but not each other
	case "resourcequota", "limitrange", "secret", "configmap", "rolebinding", "clusterrolebinding", "persistentvolumeclaim", "ingress":
		return 2
	// Same deal, next layer
	case "daemonset", "deployment", "replicationcontroller", "replicaset", "job", "cronjob", "statefulset":
		return 3
	// Assumption: anything not mentioned isn't depended _upon_, so
	// can come last.
	default:
		return 4
	}
}

type applyOrder []applyObject

func (objs applyOrder) Len() int {
	return len(objs)
}

func (objs applyOrder) Swap(i, j int) {
	objs[i], objs[j] = objs[j], objs[i]
}

func (objs applyOrder) Less(i, j int) bool {
	_, ki, ni := objs[i].ResourceID.Components()
	_, kj, nj := objs[j].ResourceID.Components()
	ranki, rankj := rankOfKind(ki), rankOfKind(kj)
	if ranki == rankj {
		return ni < nj
	}
	return ranki < rankj
}

func (c *Kubectl) apply(logger log.Logger, cs changeSet, errored map[resource.ID]error) (errs cluster.SyncError) {
	f := func(objs []applyObject, cmd string, args ...string) {
		if len(objs) == 0 {
			return
		}
		logger.Log("cmd", cmd, "args", strings.Join(args, " "), "count", len(objs))
		args = append(args, cmd)

		var multi, single []applyObject
		if len(errored) == 0 {
			multi = objs
		} else {
			for _, obj := range objs {
				if _, ok := errored[obj.ResourceID]; ok {
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
			r := bytes.NewReader(obj.Payload)
			if err := c.doCommand(logger, r, args...); err != nil {
				errs = append(errs, cluster.ResourceError{
					ResourceID: obj.ResourceID,
					Source:     obj.Source,
					Error:      err,
				})
			}
		}
	}

	// When deleting objects, the only real concern is that we don't
	// try to delete things that have already been deleted by
	// Kubernetes' GC -- most notably, resources in a namespace which
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

func makeMultidoc(objs []applyObject) *bytes.Buffer {
	buf := &bytes.Buffer{}
	for _, obj := range objs {
		appendYAMLToBuffer(obj.Payload, buf)
	}
	return buf
}

func (c *Kubectl) kubectlCommand(args ...string) *exec.Cmd {
	return exec.Command(c.exe, append(c.connectArgs(), args...)...)
}
