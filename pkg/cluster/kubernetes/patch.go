package kubernetes

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/evanphx/json-patch"
	jsonyaml "github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"

	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
	"github.com/fluxcd/flux/pkg/resource"
)

func createManifestPatch(originalManifests, modifiedManifests []byte, originalSource, modifiedSource string) ([]byte, error) {
	originalResources, err := kresource.ParseMultidoc(originalManifests, originalSource)
	if err != nil {
		fmt.Errorf("cannot parse %s: %s", originalSource, err)
	}

	modifiedResources, err := kresource.ParseMultidoc(modifiedManifests, modifiedSource)
	if err != nil {
		fmt.Errorf("cannot parse %s: %s", modifiedSource, err)
	}
	// Sort output by resource identifiers
	var originalIDs []string
	for id, _ := range originalResources {
		originalIDs = append(originalIDs, id)
	}
	sort.Strings(originalIDs)

	buf := bytes.NewBuffer(nil)
	scheme := getFullScheme()
	for _, id := range originalIDs {
		originalResource := originalResources[id]
		modifiedResource, ok := modifiedResources[id]
		if !ok {
			// Only generate patches for resources present in both files
			continue
		}
		patch, err := getPatch(originalResource, modifiedResource, scheme)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain patch for resource %s: %s", id, err)
		}
		if bytes.Equal(patch, []byte("{}\n")) {
			// Avoid outputting empty patches
			continue
		}
		if err := appendYAMLToBuffer(patch, buf); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func applyManifestPatch(originalManifests, patchManifests []byte, originalSource, patchSource string) ([]byte, error) {
	originalResources, err := kresource.ParseMultidoc(originalManifests, originalSource)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %s", originalSource, err)
	}

	patchResources, err := kresource.ParseMultidoc(patchManifests, patchSource)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %s", patchSource, err)
	}

	// Make sure all patch resources have a matching resource
	for id, patchResource := range patchResources {
		if _, ok := originalResources[id]; !ok {
			return nil, fmt.Errorf("patch refers to missing resource (%s)", resourceID(patchResource))
		}
	}

	// Sort output by resource identifiers
	var originalIDs []string
	for id, _ := range originalResources {
		originalIDs = append(originalIDs, id)
	}
	sort.Strings(originalIDs)

	buf := bytes.NewBuffer(nil)
	scheme := getFullScheme()
	for _, id := range originalIDs {
		originalResource := originalResources[id]
		resourceBytes := originalResource.Bytes()
		if patchedResource, ok := patchResources[id]; ok {
			// There was a patch, apply it
			patched, err := applyPatch(originalResource, patchedResource, scheme)
			if err != nil {
				return nil, fmt.Errorf("cannot apply patch for resource %s: %s", id, err)
			}
			resourceBytes = patched
		}
		if err := appendYAMLToBuffer(resourceBytes, buf); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func getFullScheme() *runtime.Scheme {
	fullScheme := runtime.NewScheme()
	utilruntime.Must(k8sscheme.AddToScheme(fullScheme))
	// HelmRelease and FluxHelmRelease are intentionally not added to the scheme.
	// This is done for two reasons:
	// 1. The kubernetes strategic merge patcher chokes on the freeform
	//    values under `values:`.
	// 2. External tools like kustomize won't be able to apply SMPs
	//    on Custom Resources, thus we use a normal jsonmerge instead.
	//
	// utilruntime.Must(fluxscheme.AddToScheme(fullScheme))
	return fullScheme
}

func getPatch(originalManifest kresource.KubeManifest, modifiedManifest kresource.KubeManifest, scheme *runtime.Scheme) ([]byte, error) {
	groupVersion, err := schema.ParseGroupVersion(originalManifest.GroupVersion())
	if err != nil {
		return nil, fmt.Errorf("cannot parse groupVersion %q: %s", originalManifest.GroupVersion(), err)
	}
	manifest1JSON, err := jsonyaml.YAMLToJSON(originalManifest.Bytes())
	if err != nil {
		return nil, fmt.Errorf("cannot transform original resource (%s) to JSON: %s",
			resourceID(originalManifest), err)
	}
	manifest2JSON, err := jsonyaml.YAMLToJSON(modifiedManifest.Bytes())
	if err != nil {
		return nil, fmt.Errorf("cannot transform modified resource (%s) to JSON: %s",
			resourceID(modifiedManifest), err)
	}
	gvk := groupVersion.WithKind(originalManifest.GetKind())
	obj, err := scheme.New(gvk)
	var patchJSON []byte
	switch {
	case runtime.IsNotRegisteredError(err):
		// try a normal JSON merge patch
		patchJSON, err = jsonpatch.CreateMergePatch(manifest1JSON, manifest2JSON)
	case err != nil:
		err = fmt.Errorf("cannot obtain scheme for GroupVersionKind %q: %s", gvk, err)
	default:
		patchJSON, err = strategicpatch.CreateTwoWayMergePatch(manifest1JSON, manifest2JSON, obj)
	}
	if err != nil {
		return nil, err
	}
	var jsonObj interface{}
	// We are using yaml.Unmarshal here (instead of json.Unmarshal) because the
	// Go JSON library doesn't try to pick the right number type (int, float,
	// etc.) when unmarshalling to interface{}
	err = yaml.Unmarshal(patchJSON, &jsonObj)
	if err != nil {
		return nil, fmt.Errorf("cannot parse patch (resource %s): %s",
			resourceID(originalManifest), err)
	}
	// Make sure the non-empty patches come with metadata so that they can be matched in multidoc yaml context
	if m, ok := jsonObj.(map[interface{}]interface{}); ok && len(m) > 0 {
		jsonObj, err = addIdentifyingData(originalManifest.GroupVersion(),
			originalManifest.GetKind(), originalManifest.GetName(), originalManifest.GetNamespace(), m)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot add metadata to patch (resource %s): %s", resourceID(originalManifest), err)
	}
	patch, err := yaml.Marshal(jsonObj)
	if err != nil {
		return nil, fmt.Errorf("cannot transform updated patch (resource %s) to YAML: %s",
			resourceID(originalManifest), err)
	}
	return patch, nil
}

func addIdentifyingData(apiVersion string, kind string, name string, namespace string,
	obj map[interface{}]interface{}) (map[interface{}]interface{}, error) {

	toMerge := map[interface{}]interface{}{}
	toMerge["apiVersion"] = apiVersion
	toMerge["kind"] = kind
	metadata := map[string]string{
		"name": name,
	}
	if len(namespace) > 0 {
		metadata["namespace"] = namespace
	}
	toMerge["metadata"] = metadata
	err := mergo.Merge(&obj, toMerge)
	return obj, err
}

func applyPatch(originalManifest, patchManifest kresource.KubeManifest, scheme *runtime.Scheme) ([]byte, error) {
	groupVersion, err := schema.ParseGroupVersion(originalManifest.GroupVersion())
	if err != nil {
		return nil, fmt.Errorf("cannot parse groupVersion %q: %s", originalManifest.GroupVersion(), err)
	}
	originalJSON, err := jsonyaml.YAMLToJSON(originalManifest.Bytes())
	if err != nil {
		return nil, fmt.Errorf("cannot transform original resource (%s) to JSON: %s",
			resourceID(originalManifest), err)
	}
	patchJSON, err := jsonyaml.YAMLToJSON(patchManifest.Bytes())
	if err != nil {
		return nil, fmt.Errorf("cannot transform patch resource (%s) to JSON: %s",
			resourceID(patchManifest), err)
	}
	obj, err := scheme.New(groupVersion.WithKind(originalManifest.GetKind()))
	var patchedJSON []byte
	switch {
	case runtime.IsNotRegisteredError(err):
		// try a normal JSON merging
		patchedJSON, err = jsonpatch.MergePatch(originalJSON, patchJSON)
	default:
		patchedJSON, err = strategicpatch.StrategicMergePatch(originalJSON, patchJSON, obj)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot patch resource %s: %s", resourceID(originalManifest), err)
	}
	patched, err := jsonyaml.JSONToYAML(patchedJSON)
	if err != nil {
		return nil, fmt.Errorf("cannot transform patched resource (%s) to YAML: %s", resourceID(originalManifest), err)
	}
	return patched, nil
}

// resourceID works like Resource.ID() but avoids <cluster> namespaces,
// since they may be incorrect
func resourceID(manifest kresource.KubeManifest) resource.ID {
	return resource.MakeID(manifest.GetNamespace(), manifest.GetKind(), manifest.GetName())
}
