package kubernetes

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"gopkg.in/yaml.v2"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
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
// "post-processing" to make sure the view of the manifests is what
// would be applied; in particular, it fills in the namespace of
// manifests that would be given a default namespace when applied.
type manifests struct {
	namespacer       namespacer
	logger           log.Logger
	resourceWarnings map[string]struct{}
	sopsEnabled      bool
}

func NewManifests(ns namespacer, logger log.Logger) *manifests {
	return &manifests{
		namespacer:       ns,
		logger:           logger,
		resourceWarnings: map[string]struct{}{},
		sopsEnabled:      false,
	}
}

func NewSopsManifests(ns namespacer, logger log.Logger) *manifests {
	return &manifests{
		namespacer:       ns,
		logger:           logger,
		resourceWarnings: map[string]struct{}{},
		sopsEnabled:      true,
	}
}

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
			m.logger.Log("info", fmt.Sprintf("found scope of resource %s, back in business!", km.ResourceID().String()))
			delete(m.resourceWarnings, resIDStr)
		}
		result[km.ResourceID().String()] = km
	}
	return result, nil
}

func (m *manifests) LoadManifests(baseDir string, paths []string) (map[string]resource.Resource, error) {
	manifests, err := kresource.Load(baseDir, paths, m.sopsEnabled)
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

func (m *manifests) SetWorkloadContainerImage(def []byte, id resource.ID, container string, image image.Ref) ([]byte, error) {
	resources, err := m.ParseManifest(def, "stdin")
	if err != nil {
		return nil, err
	}
	res, ok := resources[id.String()]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", id.String())
	}
	// Check if the workload is a HelmRelease, and try to resolve an image
	// map for the given container to perform an update based on mapped YAML
	// dot notation paths. If resolving the map fails (either because there
	// is no map for the given container, or the mapping does not resolve
	// in to a valid image ref), it falls through and attempts to update
	// using just the container name (as it must origin from an automated
	// detection).
	//
	// NB: we do this here and not in e.g. the `resource` package, to ensure
	// everything _outside_ this package only knows about Kubernetes native
	// containers, and not the dot notation YAML paths we invented for custom
	// Helm value structures.
	if hr, ok := res.(*kresource.HelmRelease); ok {
		if paths, err := hr.GetContainerImageMap(container); err == nil {
			return updateWorkloadImagePaths(def, id, paths, image)
		}
	}
	return updateWorkloadContainer(def, id, container, image)
}

func (m *manifests) CreateManifestPatch(originalManifests, modifiedManifests []byte, originalSource, modifiedSource string) ([]byte, error) {
	return createManifestPatch(originalManifests, modifiedManifests, originalSource, modifiedSource)
}

func (m *manifests) ApplyManifestPatch(originalManifests, patchManifests []byte, originalSource, patchSource string) ([]byte, error) {
	return applyManifestPatch(originalManifests, patchManifests, originalSource, patchSource)
}

func (m *manifests) AppendManifestToBuffer(manifest []byte, buffer *bytes.Buffer) error {
	return appendYAMLToBuffer(manifest, buffer)
}

func appendYAMLToBuffer(manifest []byte, buffer *bytes.Buffer) error {
	separator := "---\n"
	bytes := buffer.Bytes()
	if len(bytes) > 0 && bytes[len(bytes)-1] != '\n' {
		separator = "\n---\n"
	}
	if _, err := buffer.WriteString(separator); err != nil {
		return fmt.Errorf("cannot write to internal buffer: %s", err)
	}
	if _, err := buffer.Write(manifest); err != nil {
		return fmt.Errorf("cannot write to internal buffer: %s", err)
	}
	return nil
}

// UpdateWorkloadPolicies in policies.go
