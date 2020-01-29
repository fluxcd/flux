package kubernetes

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/ryanuber/go-glob"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// Sync takes a definition of what should be running in the cluster,
// and attempts to make the cluster conform. An error return does not
// necessarily indicate complete failure; some resources may succeed
// in being synced, and some may fail (for example, they may be
// malformed).
func (c *Cluster) Sync(syncSet cluster.SyncSet) error {
	logger := log.With(c.logger, "method", "Sync")

	// Keep track of the checksum of each resource, so we can compare
	// them during garbage collection.
	checksums := map[string]string{}

	// NB we get all resources, since we care about leaving unsynced,
	// _ignored_ resources alone.
	clusterResources, err := c.getAllowedResourcesBySelector("")
	if err != nil {
		return errors.Wrap(err, "collating resources in cluster for sync")
	}

	cs := makeChangeSet()
	var errs cluster.SyncError
	var excluded []string
	for _, res := range syncSet.Resources {
		resID := res.ResourceID()
		id := resID.String()
		if !c.IsAllowedResource(resID) {
			excluded = append(excluded, id)
			continue
		}
		// make a record of the checksum, whether we stage it to
		// be applied or not, so that we don't delete it later.
		csum := sha1.Sum(res.Bytes())
		checkHex := hex.EncodeToString(csum[:])
		checksums[id] = checkHex
		if res.Policies().Has(policy.Ignore) {
			logger.Log("info", "not applying resource; ignore annotation in file", "resource", res.ResourceID(), "source", res.Source())
			continue
		}
		// It's possible to give a cluster resource the "ignore"
		// annotation directly -- e.g., with `kubectl annotate` -- so
		// we need to examine the cluster resource here too.
		if cres, ok := clusterResources[id]; ok && cres.Policies().Has(policy.Ignore) {
			logger.Log("info", "not applying resource; ignore annotation in cluster resource", "resource", cres.ResourceID())
			continue
		}
		resBytes, err := applyMetadata(res, syncSet.Name, checkHex)
		if err == nil {
			cs.stage("apply", res.ResourceID(), res.Source(), resBytes)
		} else {
			errs = append(errs, cluster.ResourceError{ResourceID: res.ResourceID(), Source: res.Source(), Error: err})
			break
		}
	}

	if len(excluded) > 0 {
		logger.Log("warning", "not applying resources; excluded by namespace constraints", "resources", strings.Join(excluded, ","))
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.muSyncErrors.RLock()
	if applyErrs := c.applier.apply(logger, cs, c.syncErrors); len(applyErrs) > 0 {
		errs = append(errs, applyErrs...)
	}
	c.muSyncErrors.RUnlock()

	if c.GC || c.DryGC {
		deleteErrs, gcFailure := c.collectGarbage(syncSet, checksums, logger, c.DryGC)
		if gcFailure != nil {
			return gcFailure
		}
		errs = append(errs, deleteErrs...)
	}

	// If `nil`, errs is a cluster.SyncError(nil) rather than error(nil), so it cannot be returned directly.
	if errs == nil {
		return nil
	}

	// It is expected that Cluster.Sync is invoked with *all* resources.
	// Otherwise it will override previously recorded sync errors.
	c.setSyncErrors(errs)
	return errs
}

func (c *Cluster) collectGarbage(
	syncSet cluster.SyncSet,
	checksums map[string]string,
	logger log.Logger,
	dryRun bool) (cluster.SyncError, error) {

	orphanedResources := makeChangeSet()

	clusterResources, err := c.getAllowedGCMarkedResourcesInSyncSet(syncSet.Name)
	if err != nil {
		return nil, errors.Wrap(err, "collating resources in cluster for calculating garbage collection")
	}

	for resourceID, res := range clusterResources {
		actual := res.GetChecksum()
		expected, ok := checksums[resourceID]

		switch {
		case !ok: // was not recorded as having been staged for application
			c.logger.Log("info", "cluster resource not in resources to be synced; deleting", "dry-run", dryRun, "resource", resourceID)
			if !dryRun {
				orphanedResources.stage("delete", res.ResourceID(), "<cluster>", res.IdentifyingBytes())
			}
		case actual != expected:
			c.logger.Log("warning", "resource to be synced has not been updated; skipping", "dry-run", dryRun, "resource", resourceID)
			continue
		default:
			// The checksum is the same, indicating that it was
			// applied earlier. Leave it alone.
		}
	}

	return c.applier.apply(logger, orphanedResources, nil), nil
}

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

func (c *Cluster) filterResources(resources *meta_v1.APIResourceList) *meta_v1.APIResourceList {
	list := []meta_v1.APIResource{}
	for _, apiResource := range resources.APIResources {
		fullName := fmt.Sprintf("%s/%s", resources.GroupVersion, apiResource.Kind)
		excluded := false
		for _, exp := range c.resourceExcludeList {
			if glob.Glob(exp, fullName) {
				excluded = true
				break
			}
		}
		if !excluded {
			list = append(list, apiResource)
		}
	}

	return &meta_v1.APIResourceList{
		TypeMeta:     resources.TypeMeta,
		GroupVersion: resources.GroupVersion,
		APIResources: list,
	}
}

func (c *Cluster) getAllowedResourcesBySelector(selector string) (map[string]*kuberesource, error) {
	listOptions := meta_v1.ListOptions{}
	if selector != "" {
		listOptions.LabelSelector = selector
	}

	sgs, err := c.client.discoveryClient.ServerGroups()
	if sgs == nil {
		return nil, err
	}

	resources := []*meta_v1.APIResourceList{}
	for i := range sgs.Groups {
		gv := sgs.Groups[i].PreferredVersion.GroupVersion

		excluded := false
		for _, exp := range c.resourceExcludeList {
			if glob.Glob(exp, fmt.Sprintf("%s/", gv)) {
				excluded = true
				break
			}
		}

		if !excluded {
			if r, err := c.client.discoveryClient.ServerResourcesForGroupVersion(gv); err == nil {
				if r != nil {
					resources = append(resources, c.filterResources(r))
				}
			} else {
				// ignore errors for resources with empty group version instead of failing to sync
				if err.Error() != fmt.Sprintf("Got empty response for: %v", gv) {
					return nil, err
				}
			}
		}
	}

	result := map[string]*kuberesource{}

	contains := func(a []string, x string) bool {
		for _, n := range a {
			if x == n {
				return true
			}
		}
		return false
	}

	for _, resource := range resources {
		for _, apiResource := range resource.APIResources {
			verbs := apiResource.Verbs
			if !contains(verbs, "list") {
				continue
			}
			groupVersion, err := schema.ParseGroupVersion(resource.GroupVersion)
			if err != nil {
				return nil, err
			}
			gvr := groupVersion.WithResource(apiResource.Name)
			list, err := c.listAllowedResources(apiResource.Namespaced, gvr, listOptions)
			if err != nil {
				if apierrors.IsForbidden(err) {
					// we are not allowed to list this resource but
					// shouldn't prevent us from listing the rest
					continue
				}
				return nil, err
			}

			for i, item := range list {
				apiVersion := item.GetAPIVersion()
				kind := item.GetKind()

				itemDesc := fmt.Sprintf("%s:%s", apiVersion, kind)
				// https://github.com/kontena/k8s-client/blob/6e9a7ba1f03c255bd6f06e8724a1c7286b22e60f/lib/k8s/stack.rb#L17-L22
				if itemDesc == "v1:ComponentStatus" || itemDesc == "v1:Endpoints" {
					continue
				}

				// exclude anything that has an ownerReference
				owners := item.GetOwnerReferences()
				if owners != nil && len(owners) > 0 {
					continue
				}

				res := &kuberesource{obj: &list[i], namespaced: apiResource.Namespaced}
				result[res.ResourceID().String()] = res
			}
		}
	}

	return result, nil
}

func (c *Cluster) listAllowedResources(
	namespaced bool, gvr schema.GroupVersionResource, options meta_v1.ListOptions) ([]unstructured.Unstructured, error) {
	if !namespaced {
		// The resource is not namespaced, everything is allowed
		resourceClient := c.client.dynamicClient.Resource(gvr)
		data, err := resourceClient.List(options)
		if err != nil {
			return nil, err
		}
		return data.Items, nil
	}

	// List resources only from the allowed namespaces
	namespaces, err := c.getAllowedAndExistingNamespaces(context.Background())
	if err != nil {
		return nil, err
	}
	var result []unstructured.Unstructured
	for _, ns := range namespaces {
		data, err := c.client.dynamicClient.Resource(gvr).Namespace(ns).List(options)
		if err != nil {
			return result, err
		}
		result = append(result, data.Items...)
	}
	return result, nil
}

func (c *Cluster) getAllowedGCMarkedResourcesInSyncSet(syncSetName string) (map[string]*kuberesource, error) {
	allGCMarkedResources, err := c.getAllowedResourcesBySelector(gcMarkLabel) // means "gcMarkLabel exists"
	if err != nil {
		return nil, err
	}
	allowedSyncSetGCMarkedResources := map[string]*kuberesource{}
	for resID, kres := range allGCMarkedResources {
		// Discard resources whose mark doesn't match their resource ID
		if kres.GetGCMark() != makeGCMark(syncSetName, resID) {
			continue
		}
		allowedSyncSetGCMarkedResources[resID] = kres
	}
	return allowedSyncSetGCMarkedResources, nil
}

func applyMetadata(res resource.Resource, syncSetName, checksum string) ([]byte, error) {
	definition := map[interface{}]interface{}{}
	if err := yaml.Unmarshal(res.Bytes(), &definition); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to parse yaml from %s", res.Source()))
	}

	mixin := map[string]interface{}{}

	if syncSetName != "" {
		mixinLabels := map[string]string{}
		mixinLabels[gcMarkLabel] = makeGCMark(syncSetName, res.ResourceID().String())
		mixin["labels"] = mixinLabels
	}

	// After loading the manifest the namespace of the resource can change
	// (e.g. a default namespace is applied)
	// The `ResourceID` should give us the up-to-date value
	// (see `KubeManifest.SetNamespace`)
	namespace, _, _ := res.ResourceID().Components()
	if namespace != kresource.ClusterScope {
		mixin["namespace"] = namespace
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
