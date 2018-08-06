package testfiles

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveworks/flux"
)

func TempDir(t *testing.T) (string, func()) {
	newDir, err := ioutil.TempDir(os.TempDir(), "flux-test")
	if err != nil {
		t.Fatal("failed to create temp directory")
	}

	cleanup := func() {
		if strings.HasPrefix(newDir, os.TempDir()) {
			if err := os.RemoveAll(newDir); err != nil {
				t.Errorf("Failed to delete %s: %v", newDir, err)
			}
		}
	}
	return newDir, cleanup
}

// WriteTestFiles ... given a directory, create files in it, based on predetermined file content
func WriteTestFiles(dir string) error {
	for name, content := range Files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return err
		}
		if err := ioutil.WriteFile(path, []byte(content), 0666); err != nil {
			return err
		}
	}
	return nil
}

// ----- DATA

// ResourceMap is the map of resource names to relative paths, which
// must correspond with `Files` below.
var ResourceMap = map[flux.ResourceID]string{
	flux.MustParseResourceID("default:deployment/helloworld"):     "helloworld-deploy.yaml",
	flux.MustParseResourceID("default:deployment/locked-service"): "locked-service-deploy.yaml",
	flux.MustParseResourceID("default:deployment/test-service"):   "test/test-service-deploy.yaml",
	flux.MustParseResourceID("default:deployment/multi-deploy"):   "multi.yaml",
	flux.MustParseResourceID("default:service/multi-service"):     "multi.yaml",
	flux.MustParseResourceID("default:deployment/list-deploy"):    "list.yaml",
	flux.MustParseResourceID("default:service/list-service"):      "list.yaml",
	flux.MustParseResourceID("default:deployment/semver"):         "semver-deploy.yaml",
}

// ServiceMap ... given a base path, construct the map representing
// the services given in the test data. Must be kept in sync with
// `Files` below. TODO(michael): derive from ResourceMap, or similar.
func ServiceMap(dir string) map[flux.ResourceID][]string {
	return map[flux.ResourceID][]string{
		flux.MustParseResourceID("default:deployment/helloworld"):     []string{filepath.Join(dir, "helloworld-deploy.yaml")},
		flux.MustParseResourceID("default:deployment/locked-service"): []string{filepath.Join(dir, "locked-service-deploy.yaml")},
		flux.MustParseResourceID("default:deployment/test-service"):   []string{filepath.Join(dir, "test/test-service-deploy.yaml")},
		flux.MustParseResourceID("default:deployment/multi-deploy"):   []string{filepath.Join(dir, "multi.yaml")},
		flux.MustParseResourceID("default:deployment/list-deploy"):    []string{filepath.Join(dir, "list.yaml")},
		flux.MustParseResourceID("default:deployment/semver"):         []string{filepath.Join(dir, "semver-deploy.yaml")},
	}
}

var Files = map[string]string{
	"garbage": "This should just be ignored, since it is not YAML",
	// Some genuine manifests
	"helloworld-deploy.yaml": `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: helloworld
spec:
  minReadySeconds: 1
  replicas: 5
  template:
    metadata:
      labels:
        name: helloworld
    spec:
      containers:
      - name: greeter
        image: quay.io/weaveworks/helloworld:master-a000001
        args:
        - -msg=Ahoy
        ports:
        - containerPort: 80
      - name: sidecar
        image: weaveworks/sidecar:master-a000001
        args:
        - -addr=:8080
        ports:
        - containerPort: 8080
`,
	// Automated deployment with semver enabled
	"semver-deploy.yaml": `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: semver
  annotations:
    flux.weave.works/automated: "true"
    flux.weave.works/tag.greeter: semver:*
spec:
  minReadySeconds: 1
  replicas: 5
  template:
    metadata:
      labels:
        name: semver
    spec:
      containers:
      - name: greeter
        image: quay.io/weaveworks/helloworld:master-a000001
        args:
        - -msg=Ahoy
        ports:
        - containerPort: 80
`,
	"locked-service-deploy.yaml": `apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  annotations:
    flux.weave.works/locked: "true"
  name: locked-service
spec:
  minReadySeconds: 1
  replicas: 5
  template:
    metadata:
      labels:
        name: locked-service
    spec:
      containers:
      - name: locked-service
        image: quay.io/weaveworks/locked-service:1
        args:
        - -msg=Ahoy
        ports:
        - containerPort: 80
`,
	"test/test-service-deploy.yaml": `apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: test-service
spec:
  minReadySeconds: 1
  replicas: 5
  template:
    metadata:
      labels:
        name: test-service
    spec:
      containers:
      - name: test-service
        image: quay.io/weaveworks/test-service:1
        args:
        - -msg=Ahoy
        ports:
        - containerPort: 80
`,
	// A multidoc, since we support those now
	"multi.yaml": `---
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  annotations:
    flux.weave.works/automated: "true"
  name: multi-deploy
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app : multi-app
    spec:
      containers:
        - name: hello
          image: quay.io/weaveworks/helloworld:master-a000001
          imagePullPolicy: Always
          ports:
          - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: multi-service
spec:
  type: NodePort
  ports:
  - port: 80
    protocol: TCP
  selector:
    app: multi-app
`,

	// A List resource
	"list.yaml": `---
apiVersion: v1
kind: List
items:
- apiVersion: apps/v1beta1
  kind: Deployment
  metadata:
    name: list-deploy
  spec:
    replicas: 1
    template:
      metadata:
        labels:
          app : list-app
      spec:
        containers:
          - name: hello
            image: quay.io/weaveworks/helloworld:master-a000001
            imagePullPolicy: Always
            ports:
            - containerPort: 80
- apiVersion: v1
  kind: Service
  metadata:
    labels:
      app: list-app
    name: list-service
  spec:
    type: NodePort
    ports:
    - port: 80
      protocol: TCP
    selector:
      app: list-app
`,

	// A tricksy chart directory, which should be skipped entirely. Adapted from
	// https://github.com/kubernetes/helm/tree/master/docs/examples
	"charts/nginx/Chart.yaml": `---
name: nginx
description: A basic NGINX HTTP server
version: 0.1.0
kubeVersion: ">=1.2.0"
keywords:
  - http
  - nginx
  - www
  - web
home: https://github.com/kubernetes/helm
sources:
  - https://hub.docker.com/_/nginx/
maintainers:
  - name: technosophos
    email: mbutcher@deis.com
`,
	"charts/nginx/values.yaml": `---
# Declare name/value pairs to be passed into your templates.
replicaCount: 1
restartPolicy: Never
index: >-
  <h1>Hello</h1>
  <p>This is a test</p>
image:
  repository: nginx
  tag: 1.11.0
  pullPolicy: IfNotPresent
`,
	"charts/nginx/templates/deployment.yaml": `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: {{ template "nginx.fullname" . }}
  labels:
    app: {{ template "nginx.name" . }}
spec:
  replicas: {{ .Values.replicaCount }}
  template:
    metadata:
{{- if .Values.podAnnotations }}
      # Allows custom annotations to be specified
      annotations:
{{ toYaml .Values.podAnnotations | indent 8 }}
{{- end }}
      labels:
        app: {{ template "nginx.name" . }}
        release: {{ .Release.Name }}
    spec:
      containers:
        - name: {{ template "nginx.name" . }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: 80
              protocol: TCP
`,
}

var FilesUpdated = map[string]string{
	"helloworld-deploy.yaml": `apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: helloworld
spec:
  minReadySeconds: 1
  replicas: 5
  template:
    metadata:
      labels:
        name: helloworld
    spec:
      containers:
      - name: greeter
        image: quay.io/weaveworks/helloworld:master-a000001
        args:
        - -msg=Ahoy2
        ports:
        - containerPort: 80
      - name: sidecar
        image: weaveworks/sidecar:master-a000002
        args:
        - -addr=:8080
        ports:
        - containerPort: 8080
`,
}
