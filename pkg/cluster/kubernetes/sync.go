package kubernetes

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
