package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
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

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes/resource"
)

func main() {
	if len(os.Args) != 3 {
		exit("usage: kubedelta <original.yaml> <updated.yaml>\n" +
			"Obtains the difference (Strategic or JSON merge patch) between original.yaml and updated.yaml.")
	}

	f1, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		exit("cannot open file %s: %s", os.Args[1], err)
	}

	resources1, err := resource.ParseMultidoc(f1, os.Args[1])
	if err != nil {
		exit("cannot parse file %s: %s", os.Args[1], err)
	}

	f2, err := ioutil.ReadFile(os.Args[2])
	if err != nil {
		exit("cannot open file %s: %s", os.Args[2], err)
	}
	resources2, err := resource.ParseMultidoc(f2, os.Args[2])
	if err != nil {
		exit("cannot parse file %s: %s", os.Args[2], err)
	}

	buf := bytes.NewBuffer(nil)

	// Sort the output by resource id
	var ids1 []string
	for id, _ := range resources1 {
		ids1 = append(ids1, id)
	}
	sort.Strings(ids1)

	scheme := getFullScheme()
	for _, id := range ids1 {
		res1 := resources1[id]
		res2, ok := resources2[id]
		if !ok {
			// Only generate patches for resources present in both files
			continue
		}
		patch, err := getPatch(res1, res2, scheme)
		if err != nil {
			exit("cannot obtain patch for resource %s: %s", id, err)
		}
		if bytes.Equal(patch, []byte("{}\n")) {
			// Avoid printing empty patches
			continue
		}
		separator := "---\n"
		bytes := buf.Bytes()
		if len(bytes) > 0 && bytes[len(bytes)-1] != '\n' {
			separator = "\n" + separator
		}
		mustWriteString(buf, separator)
		mustWriteBytes(buf, patch)
	}

	if _, err = buf.WriteTo(os.Stdout); err != nil {
		exit("cannot write result to stdout: %s", err)
	}
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

func getPatch(manifest1 resource.KubeManifest, manifest2 resource.KubeManifest, scheme *runtime.Scheme) ([]byte, error) {
	groupVersion, err := schema.ParseGroupVersion(manifest1.GroupVersion())
	if err != nil {
		return nil, fmt.Errorf("cannot parse groupVersion %q: %s", manifest1.GroupVersion(), err)
	}
	manifest1JSON, err := jsonyaml.YAMLToJSON(manifest1.Bytes())
	if err != nil {
		return nil, fmt.Errorf("cannot transform original resource (%s) to JSON: %s",
			resourceID(manifest1), err)
	}
	manifest2JSON, err := jsonyaml.YAMLToJSON(manifest2.Bytes())
	if err != nil {
		return nil, fmt.Errorf("cannot transform updated resource (%s) to JSON: %s",
			resourceID(manifest2), err)
	}
	gvk := groupVersion.WithKind(manifest1.GetKind())
	obj, err := scheme.New(gvk)
	var patchJSON []byte
	switch {
	case runtime.IsNotRegisteredError(err):
		// try a normal JSON merge patch
		patchJSON, err = jsonpatch.CreateMergePatch(manifest1JSON, manifest2JSON)
	case err != nil:
		err = fmt.Errorf("cannot obtain scheme for gvk %q: %s", gvk, err)
	default:
		patchJSON, err = strategicpatch.CreateTwoWayMergePatch(manifest1JSON, manifest2JSON, obj)
	}
	if err != nil {
		return nil, err
	}
	var jsonObj interface{}
	err = yaml.Unmarshal(patchJSON, &jsonObj)
	if err != nil {
		return nil, fmt.Errorf("cannot transform updated patch to YAML: %s", err)
	}
	// Make sure the non-empty patches come with metadata so that they can be matched from a multidoc yaml stream
	if m, ok := jsonObj.(map[interface{}]interface{}); ok && len(m) > 0 {
		jsonObj, err = addIdentifyingData(manifest1.GroupVersion(), manifest1.GetKind(), manifest1.GetName(), manifest1.GetNamespace(), m)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot add metadata to patch (resource %s): %s",
			resourceID(manifest1), err)
	}
	patch, err := yaml.Marshal(jsonObj)
	if err != nil {
		return nil, fmt.Errorf("cannot transform updated patch (resource %s) to YAML: %s",
			resourceID(manifest1), err)
	}
	return patch, nil
}

func addIdentifyingData(apiVersion string, kind string, name string, namespace string, obj map[interface{}]interface{}) (map[interface{}]interface{}, error) {
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

func exit(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func mustWriteString(buf *bytes.Buffer, s string) {
	if _, err := buf.WriteString(s); err != nil {
		exit("cannot write to internal buffer: %s", err)
	}
}

func mustWriteBytes(buf *bytes.Buffer, bytes []byte) {
	if _, err := buf.Write(bytes); err != nil {
		exit("cannot write to internal buffer: %s", err)
	}
}

// resourceID works like ResourceID() but avoids <cluster> namespaces,
// since they may be incorrect
func resourceID(manifest resource.KubeManifest) flux.ResourceID {
	return flux.MakeResourceID(manifest.GetNamespace(), manifest.GetKind(), manifest.GetKind())
}
