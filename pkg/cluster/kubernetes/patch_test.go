package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
)

func TestPatchAndApply(t *testing.T) {
	for _, entry := range []struct {
		original string
		modified string
	}{
		{ // unmodified
			original: `apiVersion: v1
kind: Namespace
metadata:
  name: namespace
`,
			modified: `apiVersion: v1
kind: Namespace
metadata:
  name: namespace
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
			modified: `apiVersion: flux.weave.works/v1beta1
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
			modified: `apiVersion: flux.weave.works/v1beta1
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
			modified: `apiVersion: apps/v1
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
			modified: `apiVersion: apps/v1
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
			modified: `apiVersion: flux.weave.works/v1beta1
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
		{ // multiple documents
			original: `apiVersion: v1
kind: Namespace
metadata:
  name: namespace
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: name
spec:
  template:
    spec:
      containers:
      - name: one
        image: one:one
---
apiVersion: flux.weave.works/v1beta1
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

			modified: `apiVersion: flux.weave.works/v1beta1
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
---
apiVersion: v1
kind: Namespace
metadata:
  name: namespace
---
apiVersion: apps/v1
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
	} {
		// Make sure creating the patch works
		patch, err := createManifestPatch([]byte(entry.original), []byte(entry.modified), "original", "updated")
		assert.NoError(t, err, "original:\n%s\n\nupdated:\n%s", entry.original, entry.modified)

		// Make sure that when applying the patch to the original manifest, we obtain the updated manifest
		patched, err := applyManifestPatch([]byte(entry.original), patch, "original", "patch")
		assert.NoError(t, err)
		expected, err := resource.ParseMultidoc([]byte(entry.modified), "updated")
		assert.NoError(t, err)
		actual, err := resource.ParseMultidoc(patched, "patched")
		assert.NoError(t, err)
		assert.Equal(t, len(actual), len(expected), "updated:\n%s\n\npatched:\n%s", entry.modified, string(patched))
		for id, expectedManifest := range expected {
			actualManifest, ok := actual[id]
			assert.True(t, ok, "resource %s missing in patched document stream", id)
			equalYAML(t, string(expectedManifest.Bytes()), string(actualManifest.Bytes()))
		}
	}
}

func equalYAML(t *testing.T, expected, actual string) {
	var obj1, obj2 interface{}
	err := yaml.Unmarshal([]byte(expected), &obj1)
	assert.NoError(t, err)
	err = yaml.Unmarshal([]byte(actual), &obj2)
	assert.NoError(t, err)
	assert.Equal(t, obj1, obj2, "expected:\n%s\n\nactual:\n%s", expected, actual)
}
