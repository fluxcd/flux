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

func WriteTestFiles(dir string) error {
	for name, content := range Files {
		path := filepath.Join(dir, name)
		if err := ioutil.WriteFile(path, []byte(content), 0666); err != nil {
			return err
		}
	}
	return nil
}

// ----- DATA

// Given a base path, construct the map representing the services
// given in the test data.
func ServiceMap(dir string) map[flux.ServiceID][]string {
	return map[flux.ServiceID][]string{
		flux.ServiceID("default/helloworld"):     []string{filepath.Join(dir, "helloworld-deploy.yaml")},
		flux.ServiceID("default/locked-service"): []string{filepath.Join(dir, "locked-service-deploy.yaml")},
		flux.ServiceID("default/test-service"):   []string{filepath.Join(dir, "test-service-deploy.yaml")},
		flux.ServiceID("default/reviews"):        []string{filepath.Join(dir, "istio-reviews-example.yaml")},
	}
}

var Files = map[string]string{
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
      - name: goodbyeworld
        image: quay.io/weaveworks/helloworld:master-a000001
        args:
        - -msg=Ahoy
        ports:
        - containerPort: 80
      - name: sidecar
        image: quay.io/weaveworks/sidecar:master-a000002
        args:
        - -addr=:8080
        ports:
        - containerPort: 8080
`,
	"helloworld-svc.yaml": `apiVersion: v1
kind: Service
metadata:
  name: helloworld
spec:
  ports:
    - port: 80
  selector:
    name: helloworld
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
	"locked-service-svc.yaml": `apiVersion: v1
kind: Service
metadata:
  name: locked-service
spec:
  ports:
    - port: 80
  selector:
    name: locked-service
`,
	"test-service-deploy.yaml": `apiVersion: extensions/v1beta1
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
	"test-service-svc.yaml": `apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  ports:
    - port: 80
  selector:
    name: test-service
`,
	"istio-reviews-example.yaml": `# Copyright 2017 Istio Authors
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

##################################################################################################
# Reviews service
##################################################################################################
apiVersion: v1
kind: Service
metadata:
  name: reviews
  labels:
    app: reviews
spec:
  ports:
  - port: 9080
    name: http
  selector:
    app: reviews
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  creationTimestamp: null
  name: reviews-v1
spec:
  replicas: 1
  strategy: {}
  template:
    metadata:
      annotations:
        alpha.istio.io/sidecar: injected
        alpha.istio.io/version: jenkins@ubuntu-16-04-build-12ac793f80be71-0.1.6-dab2033
        pod.beta.kubernetes.io/init-containers: '[{"args":["-p","15001","-u","1337"],"image":"istio/init:0.1","imagePullPolicy":"Always","name":"init","securityContext":{"capabilities":{"add":["NET_ADMIN"]}}},{"args":["-c","sysctl
          -w kernel.core_pattern=/tmp/core.%e.%p.%t \u0026\u0026 ulimit -c unlimited"],"command":["/bin/sh"],"image":"alpine","imagePullPolicy":"Always","name":"enable-core-dump","securityContext":{"privileged":true}}]'
      creationTimestamp: null
      labels:
        app: reviews
        version: v1
    spec:
      containers:
      - image: istio/examples-bookinfo-reviews-v1
        imagePullPolicy: IfNotPresent
        name: reviews
        ports:
        - containerPort: 9080
        resources: {}
      - args:
        - proxy
        - sidecar
        - -v
        - "2"
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        image: istio/proxy_debug:0.1
        imagePullPolicy: Always
        name: proxy
        resources: {}
        securityContext:
          runAsUser: 1337
        volumeMounts:
        - mountPath: /etc/certs
          name: istio-certs
          readOnly: true
      volumes:
      - name: istio-certs
        secret:
          secretName: istio.default
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  creationTimestamp: null
  name: reviews-v2
spec:
  replicas: 1
  strategy: {}
  template:
    metadata:
      annotations:
        alpha.istio.io/sidecar: injected
        alpha.istio.io/version: jenkins@ubuntu-16-04-build-12ac793f80be71-0.1.6-dab2033
        pod.beta.kubernetes.io/init-containers: '[{"args":["-p","15001","-u","1337"],"image":"istio/init:0.1","imagePullPolicy":"Always","name":"init","securityContext":{"capabilities":{"add":["NET_ADMIN"]}}},{"args":["-c","sysctl
          -w kernel.core_pattern=/tmp/core.%e.%p.%t \u0026\u0026 ulimit -c unlimited"],"command":["/bin/sh"],"image":"alpine","imagePullPolicy":"Always","name":"enable-core-dump","securityContext":{"privileged":true}}]'
      creationTimestamp: null
      labels:
        app: reviews
        version: v2
    spec:
      containers:
      - image: istio/examples-bookinfo-reviews-v2
        imagePullPolicy: IfNotPresent
        name: reviews
        ports:
        - containerPort: 9080
        resources: {}
      - args:
        - proxy
        - sidecar
        - -v
        - "2"
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        image: istio/proxy_debug:0.1
        imagePullPolicy: Always
        name: proxy
        resources: {}
        securityContext:
          runAsUser: 1337
        volumeMounts:
        - mountPath: /etc/certs
          name: istio-certs
          readOnly: true
      volumes:
      - name: istio-certs
        secret:
          secretName: istio.default
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  creationTimestamp: null
  name: reviews-v3
spec:
  replicas: 1
  strategy: {}
  template:
    metadata:
      annotations:
        alpha.istio.io/sidecar: injected
        alpha.istio.io/version: jenkins@ubuntu-16-04-build-12ac793f80be71-0.1.6-dab2033
        pod.beta.kubernetes.io/init-containers: '[{"args":["-p","15001","-u","1337"],"image":"istio/init:0.1","imagePullPolicy":"Always","name":"init","securityContext":{"capabilities":{"add":["NET_ADMIN"]}}},{"args":["-c","sysctl
          -w kernel.core_pattern=/tmp/core.%e.%p.%t \u0026\u0026 ulimit -c unlimited"],"command":["/bin/sh"],"image":"alpine","imagePullPolicy":"Always","name":"enable-core-dump","securityContext":{"privileged":true}}]'
      creationTimestamp: null
      labels:
        app: reviews
        version: v3
    spec:
      containers:
      - image: istio/examples-bookinfo-reviews-v3
        imagePullPolicy: IfNotPresent
        name: reviews
        ports:
        - containerPort: 9080
        resources: {}
      - args:
        - proxy
        - sidecar
        - -v
        - "2"
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        image: istio/proxy_debug:0.1
        imagePullPolicy: Always
        name: proxy
        resources: {}
        securityContext:
          runAsUser: 1337
        volumeMounts:
        - mountPath: /etc/certs
          name: istio-certs
          readOnly: true
      volumes:
      - name: istio-certs
        secret:
          secretName: istio.default
`,
}
