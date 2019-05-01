package kubernetes

import (
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"gopkg.in/yaml.v2"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

// ResourceScopes maps resource definitions (GroupVersionKind) to whether they are namespaced or not
type ResourceScopes map[schema.GroupVersionKind]v1beta1.ResourceScope

// namespacer assigns namespaces to manifests that need it (or "" if
// the manifest should not have a namespace.
type namespacer interface {
	// EffectiveNamespace gives the namespace that would be used were
	// the manifest to be applied. This may be "", indicating that it
	// should not have a namespace (i.e., it's a cluster-level
	// resource).
	EffectiveNamespace(manifest kresource.KubeManifest, knownScopes ResourceScopes) (string, error)
}

// manifests is an implementation of cluster.Manifests, particular to
// Kubernetes. Aside from loading manifests from files, it does some
// "post-processsing" to make sure the view of the manifests is what
// would be applied; in particular, it fills in the namespace of
// manifests that would be given a default namespace when applied.
type manifests struct {
	namespacer       namespacer
	logger           log.Logger
	resourceWarnings map[string]struct{}
}

func NewManifests(ns namespacer, logger log.Logger) *manifests {
	return &manifests{
		namespacer:       ns,
		logger:           logger,
		resourceWarnings: map[string]struct{}{},
	}
}

var _ cluster.Manifests = &manifests{}

func getCRDScopes(manifests map[string]kresource.KubeManifest) ResourceScopes {
	result := ResourceScopes{}
	for _, km := range manifests {
		if km.GetKind() == "CustomResourceDefinition" {
			var crd v1beta1.CustomResourceDefinition
			if err := yaml.Unmarshal(km.Bytes(), &crd); err != nil {
				// The CRD can't be parsed, so we (intentionally) ignore it and
				// just hope for EffectiveNamespace() to find its scope in the cluster if needed.
				continue
			}
			crdVersions := crd.Spec.Versions
			if len(crdVersions) == 0 {
				crdVersions = []v1beta1.CustomResourceDefinitionVersion{{Name: crd.Spec.Version}}
			}
			for _, crdVersion := range crdVersions {
				gvk := schema.GroupVersionKind{
					Group:   crd.Spec.Group,
					Version: crdVersion.Name,
					Kind:    crd.Spec.Names.Kind,
				}
				result[gvk] = crd.Spec.Scope
			}
		}
	}
	return result
}

func (m *manifests) setEffectiveNamespaces(manifests map[string]kresource.KubeManifest) (map[string]resource.Resource, error) {
	knownScopes := getCRDScopes(manifests)
	result := map[string]resource.Resource{}
	for _, km := range manifests {
		resID := km.ResourceID()
		resIDStr := resID.String()
		ns, err := m.namespacer.EffectiveNamespace(km, knownScopes)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				// discard the resource and keep going after making sure we logged about it
				if _, warningLogged := m.resourceWarnings[resIDStr]; !warningLogged {
					_, kind, name := resID.Components()
					partialResIDStr := kind + "/" + name
					m.logger.Log(
						"warn", fmt.Sprintf("cannot find scope of resource %s: %s", partialResIDStr, err),
						"impact", fmt.Sprintf("resource %s will be excluded until its scope is available", partialResIDStr))
					m.resourceWarnings[resIDStr] = struct{}{}
				}
				continue
			}
			return nil, err
		}
		km.SetNamespace(ns)
		if _, warningLogged := m.resourceWarnings[resIDStr]; warningLogged {
			// indicate that we found the resource's scope and allow logging a warning again
			m.logger.Log("info", fmt.Sprintf("found scope of resource %s, back in bussiness!", km.ResourceID().String()))
			delete(m.resourceWarnings, resIDStr)
		}
		result[km.ResourceID().String()] = km
	}
	return result, nil
}

func (m *manifests) LoadManifests(baseDir string, paths []string) (map[string]resource.Resource, error) {
	manifests, err := kresource.Load(baseDir, paths)
	if err != nil {
		return nil, err
	}
	return m.setEffectiveNamespaces(manifests)
}

func (m *manifests) ParseManifest(def []byte, source string) (map[string]resource.Resource, error) {
	resources, err := kresource.ParseMultidoc(def, source)
	if err != nil {
		return nil, err
	}
	// Note: setEffectiveNamespaces() won't work for CRD instances whose CRD is yet to be created
	// (due to the CRD not being present in kresources).
	// We could get out of our way to fix this (or give a better error) but:
	// 1. With the exception of HelmReleases CRD instances are not workloads anyways.
	// 2. The problem is eventually fixed by the first successful sync.
	result, err := m.setEffectiveNamespaces(resources)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manifests) SetWorkloadContainerImage(def []byte, id flux.ResourceID, container string, image image.Ref) ([]byte, error) {
	return updateWorkload(def, id, container, image)
}

func (m *manifests) CreateManifestPatch(originalManifests, modifiedManifests []byte, originalSource, modifiedSource string) ([]byte, error) {
	return CreateManifestPatch(originalManifests, modifiedManifests, originalSource, modifiedSource)
}

func (m *manifests) ApplyManifestPatch(originalManifests, patchManifests []byte, originalSource, patchSource string) ([]byte, error) {
	return ApplyManifestPatch(originalManifests, patchManifests, originalSource, patchSource)
}

// UpdateWorkloadPolicies in policies.go
