package testfiles

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxcd/flux/pkg/resource"
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
  return writeFiles(dir, Files)
}

// WriteSopsEncryptedTestFiles ... given a directory, create files in it, based on predetermined file content.
// These files are encrypted with sops using TestPrivateKey
func WriteSopsEncryptedTestFiles(dir string) error {
  return writeFiles(dir, SopsEncryptedFiles)
}

func writeFiles(dir string, files map[string]string) error {
  for name, content := range files {
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
var ResourceMap = map[resource.ID]string{
	resource.MustParseID("default:deployment/helloworld"):     "helloworld-deploy.yaml",
	resource.MustParseID("default:deployment/locked-service"): "locked-service-deploy.yaml",
	resource.MustParseID("default:deployment/test-service"):   "test/test-service-deploy.yaml",
	resource.MustParseID("default:deployment/multi-deploy"):   "multi.yaml",
	resource.MustParseID("default:service/multi-service"):     "multi.yaml",
	resource.MustParseID("default:deployment/list-deploy"):    "list.yaml",
	resource.MustParseID("default:service/list-service"):      "list.yaml",
	resource.MustParseID("default:deployment/semver"):         "semver-deploy.yaml",
	resource.MustParseID("default:daemonset/init"):            "init.yaml",
}

// WorkloadMap ... given a base path, construct the map representing
// the services given in the test data. Must be kept in sync with
// `Files` below. TODO(michael): derive from ResourceMap, or similar.
func WorkloadMap(dir string) map[resource.ID][]string {
	return map[resource.ID][]string{
		resource.MustParseID("default:deployment/helloworld"):     []string{filepath.Join(dir, "helloworld-deploy.yaml")},
		resource.MustParseID("default:deployment/locked-service"): []string{filepath.Join(dir, "locked-service-deploy.yaml")},
		resource.MustParseID("default:deployment/test-service"):   []string{filepath.Join(dir, "test/test-service-deploy.yaml")},
		resource.MustParseID("default:deployment/multi-deploy"):   []string{filepath.Join(dir, "multi.yaml")},
		resource.MustParseID("default:deployment/list-deploy"):    []string{filepath.Join(dir, "list.yaml")},
		resource.MustParseID("default:deployment/semver"):         []string{filepath.Join(dir, "semver-deploy.yaml")},
		resource.MustParseID("default:daemonset/init"):            []string{filepath.Join(dir, "init.yaml")},
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

	// A daemonset using initContainers
	"init.yaml": `---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: init
spec:
  template:
    spec:
      initContainers:
      - name: greeter
        image: quay.io/weaveworks/helloworld:master-a000001
      containers:
      - name: unimportant
        image: alpine:1.0
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

var SopsEncryptedFiles = map[string]string{
  "garbage": "This should just be ignored, since it is not YAML",
  "helloworld-deploy.yaml": `apiVersion: ENC[AES256_GCM,data:N/68Js00AtWIvks/pt+be5AW,iv:9Ke36D3faRNrMzm82Z9ETl3lOMhhWy8fh907K5e2Ar4=,tag:EfAzs1AQvLRH/tIQ+iZttw==,type:str]
kind: ENC[AES256_GCM,data:nLQbX7tJ0toD8A==,iv:0YTwaHF/2ltg+0ZBJnVVwGpqC3hwetUEp7VqsTmG/dc=,tag:UwzBg+T6341hwFNE72rY1w==,type:str]
metadata:
    name: ENC[AES256_GCM,data:1crujILtUO9ytA==,iv:M/ITcPrW08mPnAdhMR1lkHe+MV43nGmB7VZbg7ATE/8=,tag:wMgBuHXhsLyoiFjzuNyD5A==,type:str]
spec:
    minReadySeconds: ENC[AES256_GCM,data:Gw==,iv:wpm9BoT6zoJK1D7FfAOUZRqqSt0vuRVZkKYTUFUwuGs=,tag:u1m4qb7KImqotaXqaJsc6g==,type:int]
    replicas: ENC[AES256_GCM,data:5Q==,iv:1SgpuA3jpf/Zw7+ITPww0sSi0LDYI/b+MGvdfshFwQk=,tag:94tHbVYNISMd4V9+VJCQ1A==,type:int]
    template:
        metadata:
            labels:
                name: ENC[AES256_GCM,data:so45IhmDRm+Fig==,iv:10uaiK6rpy+vcrSC7gtuI92l3D/Zihsh1gUHPNaVxok=,tag:Ebg7aGnYo10gST02+uKOuQ==,type:str]
        spec:
            containers:
            -   name: ENC[AES256_GCM,data:o//iDf99xA==,iv:yuxcUqVo+rq83MCsxhcSOaNzHBNiTBVJkyBxjFuk/SE=,tag:iieWOQKPGYWyb+9ghEgmuw==,type:str]
                image: ENC[AES256_GCM,data:1ebh8sBwMl+7p1q0fk2FMoZ0BS6phtce+U2tftHIdzOOFwYQFPmsCnw/7LQ=,iv:QyDkmXqp9h1KJWVCBfqGxPrGcVBhzo64ZBvcV38IYls=,tag:jblN9ZU/pfkWQqK0he+jyQ==,type:str]
                args:
                - ENC[AES256_GCM,data:ny9R+PJtel8d,iv:mzsX/5JoK58pDvTpt0BM4hac1bcm41RbiVcr2Zuj7gw=,tag:d61S57ziH6B/L6icOnxvgw==,type:str]
                ports:
                -   containerPort: ENC[AES256_GCM,data:7PA=,iv:CrWgn0+5qFYtHZE4FvnFRo3D2sedD4W7GRWHYc3kXU0=,tag:xkFX+yT4NaZRjvKchypWvg==,type:int]
            -   name: ENC[AES256_GCM,data:jyAYi3UjxQ==,iv:A4yZeV7Paf8OPccs3UOzBzJUV1s2NUOdmz9XJZcV/yk=,tag:lKNkPY6OqZtI+SMhj7JR5w==,type:str]
                image: ENC[AES256_GCM,data:s0oEdBYDCG/eCsujJwHYi63q6zrc/GpKe4/OKUx4ZJEH,iv:JPgMUJfH622MpzgnNfjyXOnuuzvv3ybhpxtiU2xlO9s=,tag:KTZ7vJRhieeaBGu4ggtTrA==,type:str]
                args:
                - ENC[AES256_GCM,data:eVhs9FqWUX0OAbo=,iv:1+KSjflXnEZZc/ykA7B1+xHxknd+8SJKlqsHiH53UQU=,tag:EvzWHtsopeunX8w2im4aIg==,type:str]
                ports:
                -   containerPort: ENC[AES256_GCM,data:9zbcIA==,iv:Pm+m/RchBryI1QkLs79Yih7SzhDq0l/SJMq3TIuT4hc=,tag:Ez3o8n5P1JrhN15MwjQ2WA==,type:int]
sops:
    kms: []
    gcp_kms: []
    azure_kv: []
    lastmodified: '2019-11-29T12:11:15Z'
    mac: ENC[AES256_GCM,data:E3f6Q0F0vdXCLhQ0CIKTLk1UmEGr94xqVThsMXqXXZaKAllcy8cduIAPA1WqKkFyu2dAFumgzBvRa69pbClk2h/K1AAnBAbErHN9H7cQVCCxNZmclS7IHBMunoiaRiY+7Oey5agwFfJskAgibXrf23ePcWLO+xfWw9dIG7Y83OE=,iv:D+2mgEqXxA7x0Drqn+2j1xY8Avb8OVfgB30o3wDJB4k=,tag:1S/zCcM9vIIxQ2CGfl9Cyw==,type:str]
    pgp:
    -   created_at: '2019-11-29T12:11:13Z'
        enc: |
            -----BEGIN PGP MESSAGE-----

            hIwDVtT8p6MQvmgBA/9uGrPbdNPT1ajHjZ0/TQXLn4eH8vHtM6qfSgVLXtO5sT8/
            t5panOKVZc7TqYWMER2yA8rHb7kzfPd7rNbJYmV7UgZfz8MtMMbHqUQrwWvoI+OU
            u521j38G/PdyNCYsF3EAuUXLzUR2ka9O1qLnepM6/fwJvipQJuNvpWfNcaQkjtJe
            AaNqL5jlmK63nQbXtbOBhCBJVP1j6821aBbIGdI8W2ryaS9ZFhKI7KAcU1spB3eG
            oQRWo/i6CZ6GxNX+BC2FCdN7v72/MUaq1iBt+eHWMJmpItZO8J2UHkFcXwLnQQ==
            =5S7f
            -----END PGP MESSAGE-----
        fp: 56D4FCA7A310BE68
    unencrypted_suffix: _unencrypted
    version: 3.5.0
`,"multi.yaml": `apiVersion: ENC[AES256_GCM,data:8yqGSWgi16xjFnNa,iv:xMZAis2SzZuK88p2+vQ1sgGrNDtxTSzKetqUQ1XX1TY=,tag:8W3UsUzuvt8BKblVh4EVjg==,type:str]
kind: ENC[AES256_GCM,data:HEzLbU4MwuXioQ==,iv:Y6kFrsqekf6fC7/cLdPhCiVcq8T8LiBoZsG9F2WjB6Q=,tag:CLlUTvzxfo2PWcb/sV4qgw==,type:str]
metadata:
    annotations:
        flux.weave.works/automated: ENC[AES256_GCM,data:E+LmLg==,iv:h0SVx9ZPDXe/MmBATqPq0OEhAddpCG6S59nmCzbJ5GQ=,tag:/Bj2utiV3y37bsFPLU6ITQ==,type:str]
    name: ENC[AES256_GCM,data:8vXvzZyuySNCvDQG,iv:J/mJkH3WKkORVLAskIXjsb+hagHpWdyAYbyW1SbEPYU=,tag:B/xYXClFHV/nsp3LWC9MGg==,type:str]
spec:
    replicas: ENC[AES256_GCM,data:Ww==,iv:0s+8Y0mjmWtu51gvT20edS8tLJeJOmPryTPdBmhShas=,tag:+7kf94hvGkJ3/A2LKSofFw==,type:int]
    template:
        metadata:
            labels:
                app: ENC[AES256_GCM,data:Gwrkpeug8TUv,iv:RFieExWfC9SEIL20FqLU3EtjQ4NB0smHGIrNwtloqCA=,tag:BjZhh/Yq94qxmV0dWijClQ==,type:str]
        spec:
            containers:
            -   name: ENC[AES256_GCM,data:bqH/S7g=,iv:ZSRqaUA4I9JzYoNHystJhJmwPGWbRuTVC33kr1Mf9Rw=,tag:P1KpeBXPe4zaDfFEsLFsKA==,type:str]
                image: ENC[AES256_GCM,data:wzR31BZNM4yqRl5SufFUlDaee3cBeNm8BHvcZg5lBJlC+mlT78ICa08+qkI=,iv:tGXMno8BVZ3wRCM5pvWvxWNUb3H4KTUilKHgy9V8pmE=,tag:VkdplBRDMsMTCWryOQoc2A==,type:str]
                imagePullPolicy: ENC[AES256_GCM,data:sGhJjMQN,iv:bEuucUaoCT0SSZ2qhHpFJNjpekUldxug8hfsaoUcvnk=,tag:nIbjalYAyJogStP8f12ecg==,type:str]
                ports:
                -   containerPort: ENC[AES256_GCM,data:P4w=,iv:VsEBrStHzce907EbAL8CLbSBFaekwu8N59qrBxUSWf0=,tag:9LcQlzSKCSB9pVDEdycJSQ==,type:int]
sops:
    kms: []
    gcp_kms: []
    azure_kv: []
    lastmodified: '2019-11-29T13:10:50Z'
    mac: ENC[AES256_GCM,data:n2LtRfzJm14Lh5NPPWMH6lQw7vLDEjAOiAwqLwiQJYBYXF84yRERnNtC+pEYNrFBJmc3IrKTePVwDWkRAX/9c3b3yWi65jqQg1dxYnVg828osOe1RG6EkBIxCnM/f31DFw1gxHIGPJtNevjmEep/xAS37iEkdFQ8aJol0yLTKac=,iv:07lpkLYonPuL8gDn0O+7c/ccws7eaYzU7ONjxas+US4=,tag:WKEpV3ulK7+YyUnnm2m9pw==,type:str]
    pgp:
    -   created_at: '2019-11-29T13:10:50Z'
        enc: |
            -----BEGIN PGP MESSAGE-----

            hIwDVtT8p6MQvmgBBACu6Eg1bFkdm/SaLa2trlVDiNVZ5v19xo/TwSAUP/K3CmlT
            UH1K65aWXF3YaD9hmXX9AS3FnmtHKTt/yLsBpFttA3k4N/4z8Itr6DbLyg0a8xo3
            zbhzJX6udTq6RcLTChUKR3HFPYMs1WtYw/9vKUrDxvosYBlH/wyX11d8Pzh919Je
            AY6ZKtw+V3lk8QQosJ6hHofOirdY9WVfgXxIEUeDA6olKp4skMo6yba79RprpSNJ
            kMqasq4FZlOZDzNl4qSyoeba5awb7jvsAQ51a/v6dNyW479U4HR7XC1qgGqvgA==
            =++AZ
            -----END PGP MESSAGE-----
        fp: 56D4FCA7A310BE68
    unencrypted_suffix: _unencrypted
    version: 3.5.0
---
apiVersion: ENC[AES256_GCM,data:V5o=,iv:8a6VgSPy9PkenvXxWwL6/YU3T00+5HVt9t27EE1kgJc=,tag:Lhk+lmVrQ28ff4oYX5RPNQ==,type:str]
kind: ENC[AES256_GCM,data:v221CKuSFg==,iv:p2aNIff3rIBnRxc1YTiPbgaUUJSeTijNqP7zy60OlsA=,tag:5CvdmVgenVkzvCm7upvivA==,type:str]
metadata:
    name: ENC[AES256_GCM,data:69PoFQWEW7GcRQzkgw==,iv:At0BENGgFgzySF2Yg6hlpoIBaVerY4V9SrYO0uNDqPk=,tag:nNS0Epk0+dFXhRKfdtLEGQ==,type:str]
spec:
    type: ENC[AES256_GCM,data:Wf6waA7KilM=,iv:LfltJfrrb69L8vUhJ5nCtqqr36v0FEJQ1WpA//Hu2xo=,tag:k7xGKs9wDghNCnXQ8Kel+Q==,type:str]
    ports:
    -   port: ENC[AES256_GCM,data:s38=,iv:xOAzQJv2KML98XH+soFAe+s2riff1V/JRSXCNb6Ra/o=,tag:dA7ZdxcuTNRUh6kTBVEqDw==,type:int]
        protocol: ENC[AES256_GCM,data:M299,iv:gh8Wl/umwcN9qVnfkZKUGO43OI97eI40tsLJChhUzzc=,tag:GBAkrQQpQqDIlczlfgVGPA==,type:str]
    selector:
        app: ENC[AES256_GCM,data:C2XEHwHfxzde,iv:ak87axygw1vVIOaF0KS2XWUdA4NLg33loAkmqCU4Vw0=,tag:tHeFtLMRWSvh4fjO96vxIg==,type:str]
sops:
    kms: []
    gcp_kms: []
    azure_kv: []
    lastmodified: '2019-11-29T13:10:50Z'
    mac: ENC[AES256_GCM,data:n2LtRfzJm14Lh5NPPWMH6lQw7vLDEjAOiAwqLwiQJYBYXF84yRERnNtC+pEYNrFBJmc3IrKTePVwDWkRAX/9c3b3yWi65jqQg1dxYnVg828osOe1RG6EkBIxCnM/f31DFw1gxHIGPJtNevjmEep/xAS37iEkdFQ8aJol0yLTKac=,iv:07lpkLYonPuL8gDn0O+7c/ccws7eaYzU7ONjxas+US4=,tag:WKEpV3ulK7+YyUnnm2m9pw==,type:str]
    pgp:
    -   created_at: '2019-11-29T13:10:50Z'
        enc: |
            -----BEGIN PGP MESSAGE-----

            hIwDVtT8p6MQvmgBBACu6Eg1bFkdm/SaLa2trlVDiNVZ5v19xo/TwSAUP/K3CmlT
            UH1K65aWXF3YaD9hmXX9AS3FnmtHKTt/yLsBpFttA3k4N/4z8Itr6DbLyg0a8xo3
            zbhzJX6udTq6RcLTChUKR3HFPYMs1WtYw/9vKUrDxvosYBlH/wyX11d8Pzh919Je
            AY6ZKtw+V3lk8QQosJ6hHofOirdY9WVfgXxIEUeDA6olKp4skMo6yba79RprpSNJ
            kMqasq4FZlOZDzNl4qSyoeba5awb7jvsAQ51a/v6dNyW479U4HR7XC1qgGqvgA==
            =++AZ
            -----END PGP MESSAGE-----
        fp: 56D4FCA7A310BE68
    unencrypted_suffix: _unencrypted
    version: 3.5.0
`,
}

var EncryptedResourceMap = map[resource.ID]string{
	resource.MustParseID("<cluster>:deployment/helloworld"):     "helloworld-deploy.yaml",
	resource.MustParseID("<cluster>:deployment/multi-deploy"):   "multi.yaml",
	resource.MustParseID("<cluster>:service/multi-service"):     "multi.yaml",
}

var TestPrivateKey = `-----BEGIN PGP PRIVATE KEY BLOCK-----

lQHYBF3hCAwBBADCAXKG8FGitaQhsfWCQv0N+f2ESEoRu7GXaXO97NvTg0RyJThM
8PFXLsGeSOBERTnQcAYqpirSGBsPItU0ZtkjMsKJcIehgJzyXIOGOuiBYOjRAg5f
o5YA+nvdfWT3SDKepPnsMBVLSMqHy1tbeiFj9JWB3nQ1hKxqSBJJWyT/nwARAQAB
AAP+M61RBXKkPDQoKTWPEQipAX0Ss5bR7BFUB+H2C6Q5FglERSd27L/NeYyh1HjT
DDxoXwZIDjo+88GqC4kaw5+VvNxz/Cr6vhMxaeYR/GEz7EJ9ojMQZS4RIs3dRcIY
tqQ1K6XvHwdn86AF8fDr89spEie/XT+ipe4g7K+E8KFDP7ECAM99XnKqDAoI5jy3
kdKqt5oFjhNDy7sPH/aPg2K1VqHCh1eVOv8lysS35WClh+JXF29T6Cfuq0OdnOrQ
exFwiKcCAO9dCGX8Ti3zt8ftlrZXMfZ9mKbeDH0THlP56FhyShJMfMtlHjM5OHRU
TZWEjoVfX+joxujHXHW4dbFZcWY6uEkB/0ac+jxJTxjkTMOZYPtWah0N+/o1aPSk
x2GR6Oc/Po6bB5ZqX1GWsHeQgay65I1Zf/E8PMHeIrhadvy+d7464duhCrQXRmx1
eCA8Zmx1eEB3ZWF2ZS53b3Jrcz6IzgQTAQgAOBYhBPGqhmR86rD1iqdGPVbU/Kej
EL5oBQJd4QgMAhsvBQsJCAcCBhUKCQgLAgQWAgMBAh4BAheAAAoJEFbU/KejEL5o
PUoEAJ11Tambrn9ypClTGnaaNrXd3V4PAOUSOoVESPymDY0QBtfC98BnHwbWAb/t
wQfsXhWC8aRYBv2W5/oXA7XDbtFyElqcsI5IJ0z5sWipnhSNrkqS3KqUidTnNnXx
56TSgLfWNbzngwqfNaFXhPvEjay/UYOJPZzfa4jZpR8iFOdY
=5y9F
-----END PGP PRIVATE KEY BLOCK-----
`
