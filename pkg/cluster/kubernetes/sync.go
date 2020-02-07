package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/cache"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
	yamlutil "github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

func unmarshalResources(resourcesByID []resource.Resource) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	for _, res := range resourcesByID {
		data, err := yamlutil.YAMLToJSON(res.Bytes())
		if err != nil {
			return nil, err
		}
		un := &unstructured.Unstructured{}
		err = json.Unmarshal(data, un)
		if err != nil {
			return nil, err
		}
		resources = append(resources, un)
	}
	return resources, nil
}

func isManagedResource(r *cache.Resource) bool {
	return r.Resource != nil
}

// Sync takes a definition of what should be running in the cluster,
// and attempts to make the cluster conform. An error return does not
// necessarily indicate complete failure; some resources may succeed
// in being synced, and some may fail (for example, they may be
// malformed).
func (c *Cluster) Sync(syncSet cluster.SyncSet) error {
	sourceById := map[string]string{}
	for _, res := range syncSet.Resources {
		sourceById[res.ResourceID().String()] = res.Source()
	}
	resources, err := unmarshalResources(syncSet.Resources)
	if err != nil {
		return errors.Wrap(err, "unmarshalling repository resources")
	}

	result, err := c.engine.Sync(context.Background(), resources, isManagedResource, syncSet.Name, c.fallbackNamespace, sync.WithResourcesFilter(
		func(key kube.ResourceKey, live *unstructured.Unstructured, target *unstructured.Unstructured) bool {
			if live != nil {
				// don't GC resources that were not deployed by Flux with the same configuration
				if target == nil && !allowedForGC(live, syncSet.Name) {
					return false
				}
				if kresource.PoliciesFromAnnotations(live.GetAnnotations()).Has(policy.Ignore) {
					return false
				}
			}
			if target != nil {
				if kresource.PoliciesFromAnnotations(target.GetAnnotations()).Has(policy.Ignore) {
					return false
				}
				labels := target.GetLabels()
				if labels == nil {
					labels = map[string]string{}
				}
				labels[gcMarkLabel] = makeGCMark(syncSet.Name, resource.MakeID(target.GetNamespace(), target.GetKind(), target.GetName()).String())
				target.SetLabels(labels)
			}
			return true
		}), sync.WithOperationSettings(false, c.GC && !c.DryGC, false, false),
	)
	if err != nil {
		return err
	}

	var resourceErrors []cluster.ResourceError
	for _, res := range result {
		if res.Status == common.ResultCodeSyncFailed {
			id := resource.MakeID(res.ResourceKey.Namespace, res.ResourceKey.Kind, res.ResourceKey.Name)
			source := ""
			if src, ok := sourceById[id.String()]; ok {
				source = src
			}
			resourceErrors = append(resourceErrors, cluster.ResourceError{
				ResourceID: id,
				Source:     source,
				Error:      errors.New(res.Message),
			})
		}
	}
	if len(resourceErrors) > 0 {
		return cluster.SyncError(resourceErrors)
	}
	return nil
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

func allowedForGC(obj *unstructured.Unstructured, syncSetName string) bool {
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
