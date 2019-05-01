package main

import (
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	jsonyaml "github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/weaveworks/flux/cluster/kubernetes/resource"
)

func TestGetPatch(t *testing.T) {

	scheme := getFullScheme()
	for _, entry := range []struct {
		original string
		updated  string
	}{
		{
			original: `apiVersion: v1
kind: Namespace
metadata:
  name: namespace
`,
			updated: `apiVersion: v1
kind: Namespace
metadata:
  name: namespace
`, // unchanged
		},
		{
			original: `apiVersion: flux.weave.works/v1beta1
kind: HelmRelease
metadata:
  name: ghost
  namespace: demo
  annotations:
    flux.weave.works/automated: "false"
    flux.weave.works/tag.chart-image: glob:1.21.*
spec:
  values:
    image: bitnami/ghost
    tag: 1.21.5-r0
`,
			updated: `apiVersion: flux.weave.works/v1beta1
kind: HelmRelease
metadata:
  name: ghost
  namespace: demo
  annotations:
    flux.weave.works/automated: "false"
    flux.weave.works/tag.chart-image: glob:1.21.*
spec:
  values:
    image: bitnami/ghost
    tag: 1.21.6
`,
		},
		{
			original: `apiVersion: flux.weave.works/v1beta1
kind: HelmRelease
metadata:
  name: name
  namespace: namespace
  annotations:
   flux.weave.works/tag.container: glob:1.4.*
spec:
  values:
    container:
      image: 
        repository: stefanprodan/podinfo
        tag: 1.4.4
`,
			updated: `apiVersion: flux.weave.works/v1beta1
kind: HelmRelease
metadata:
  name: name
  namespace: namespace
  annotations:
   flux.weave.works/tag.container: glob:1.4.*
spec:
  values:
    container:
      image: 
        repository: stefanprodan/podinfo
        tag: 1.6
`,
		},
		{
			original: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: name
spec:
  template:
    spec:
      containers:
      - name: one
        image: one:one
      - name: two
        image: two:two
      initContainers:
      - name: one
        image: one:one
`,
			updated: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: name
spec:
  template:
    spec:
      containers:
      - name: one
        image: oneplus:oneplus
      - name: two
        image: two:two
      initContainers:
      - name: one
        image: one:one
`,
		},
		{
			original: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: name
spec:
  template:
    spec:
      containers:
      - name: one
        image: one:one
`,
			updated: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: name
  annotations:
    flux.weave.works/locked: "true"
spec:
  template:
    spec:
      containers:
      - name: one
        image: oneplus:oneplus
`,
		},
		{
			original: `apiVersion: flux.weave.works/v1beta1
kind: HelmRelease
metadata:
  name: ghost
  namespace: demo
  annotations:
    flux.weave.works/automated: "false"
    flux.weave.works/tag.chart-image: glob:1.21.*
spec:
  values:
    image: bitnami/ghost
    tag: 1.21.5-r0
`,
			updated: `apiVersion: flux.weave.works/v1beta1
kind: HelmRelease
metadata:
  name: ghost
  namespace: demo
  annotations:
    flux.weave.works/automated: "true"
spec:
  values:
    image: bitnami/ghost
    tag: 1.21.6
`,
		},
	} {
		// Make sure creating the patch works
		original := mustParseManifest(t, entry.original)
		updated := mustParseManifest(t, entry.updated)
		patch, err := getPatch(original, updated, scheme)
		assert.NoError(t, err, "original:\n%s\n\nupdated:\n%s", entry.original, entry.updated)

		// Make sure that when applying the patch to the original manifest, we obtain the updated manifest
		patched := applyPatch(t, original, patch, scheme)
		equalYAML(t, entry.updated, string(patched))
	}

}

func applyPatch(t *testing.T, manifest resource.KubeManifest, patch []byte, scheme *runtime.Scheme) []byte {
	groupVersion, err := schema.ParseGroupVersion(manifest.GroupVersion())
	assert.NoError(t, err)
	originalJSON, err := jsonyaml.YAMLToJSON(manifest.Bytes())
	assert.NoError(t, err)
	patchJSON, err := jsonyaml.YAMLToJSON(patch)
	assert.NoError(t, err)
	obj, err := scheme.New(groupVersion.WithKind(manifest.GetKind()))
	var patchedJSON []byte
	switch {
	case runtime.IsNotRegisteredError(err):
		// try a normal JSON merging
		patchedJSON, err = jsonpatch.MergePatch(originalJSON, patchJSON)
	default:
		patchedJSON, err = strategicpatch.StrategicMergePatch(originalJSON, patchJSON, obj)
	}
	assert.NoError(t, err)
	patched, err := jsonyaml.JSONToYAML(patchedJSON)
	assert.NoError(t, err)
	return patched
}

func equalYAML(t *testing.T, yaml1, yaml2 string) {
	var obj1, obj2 interface{}
	err := yaml.Unmarshal([]byte(yaml1), &obj1)
	assert.NoError(t, err)
	err = yaml.Unmarshal([]byte(yaml2), &obj2)
	assert.NoError(t, err)
	assert.Equal(t, obj1, obj2)
}

func mustParseManifest(t *testing.T, manifest string) resource.KubeManifest {
	manifests, err := resource.ParseMultidoc([]byte(manifest), "test")
	assert.NoError(t, err)
	assert.Len(t, manifests, 1)
	var result resource.KubeManifest
	for _, v := range manifests {
		result = v
	}
	return result
}
