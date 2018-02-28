package kubernetes

import (
	"bytes"
	"testing"

	"fmt"
	"os"

	"github.com/weaveworks/flux/image"
)

type update struct {
	name            string
	containers      []string
	updatedImage    string
	caseIn, caseOut string
}

func testUpdate(t *testing.T, u update) {
	id, err := image.ParseRef(u.updatedImage)
	if err != nil {
		t.Fatal(err)
	}

	manifest := u.caseIn
	for _, container := range u.containers {
		var out bytes.Buffer
		if err := tryUpdate([]byte(manifest), container, id, &out); err != nil {
			fmt.Fprintln(os.Stderr, "Failed:", u.name)
			t.Fatal(err)
		}
		manifest = out.String()
	}
	if manifest != u.caseOut {
		fmt.Fprintln(os.Stderr, "Failed:", u.name)
		t.Fatalf("Did not get expected result:\n\n%s\n\nInstead got:\n\n%s", u.caseOut, manifest)
	}

}

func TestUpdates(t *testing.T) {
	for _, c := range []update{
		{"common case", case1container, case1image, case1, case1out},
		{"new version like number", case2container, case2image, case2, case2out},
		{"old version like number", case2container, case2reverseImage, case2out, case2},
		{"name label out of order", case3container, case3image, case3, case3out},
		{"version (tag) with dots", case4container, case4image, case4, case4out},
		{"minimal dockerhub image name", case5container, case5image, case5, case5out},
		{"reordered keys", case6containers, case6image, case6, case6out},
		{"from prod", case7containers, case7image, case7, case7out},
	} {
		testUpdate(t, c)
	}
}

func TestUpdateList(t *testing.T) {
	for _, c := range []update{
		{"single list", listImageContainer, listImageA, singleList, singleListOut},
	} {
		testUpdate(t, c)
	}
}

// Unusual but still valid indentation between containers: and the
// next line
const case1 = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: pr-assigner
  namespace: extra
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: pr-assigner
    spec:
      imagePullSecrets:
      - name: quay-secret
      containers:
        - name: pr-assigner
          image: quay.io/weaveworks/pr-assigner:master-6f5e816
          imagePullPolicy: IfNotPresent
          args:
            - --conf_path=/config/pr-assigner.json
          env:
            - name: GITHUB_TOKEN
              valueFrom:
                secretKeyRef:
                  name: pr-assigner
                  key: githubtoken
          volumeMounts:
            - name: config-volume
              mountPath: /config
      volumes:
        - name: config-volume
          configMap:
            name: pr-assigner
            items:
              - key: conffile
                path: pr-assigner.json
`

const case1image = "quay.io/weaveworks/pr-assigner:master-1234567"

var case1container = []string{"pr-assigner"}

const case1out = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: pr-assigner
  namespace: extra
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: pr-assigner
    spec:
      imagePullSecrets:
      - name: quay-secret
      containers:
        - name: pr-assigner
          image: quay.io/weaveworks/pr-assigner:master-1234567
          imagePullPolicy: IfNotPresent
          args:
            - --conf_path=/config/pr-assigner.json
          env:
            - name: GITHUB_TOKEN
              valueFrom:
                secretKeyRef:
                  name: pr-assigner
                  key: githubtoken
          volumeMounts:
            - name: config-volume
              mountPath: /config
      volumes:
        - name: config-volume
          configMap:
            name: pr-assigner
            items:
              - key: conffile
                path: pr-assigner.json
`

// Version looks like a number
const case2 = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: fluxy
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: fluxy
        version: master-a000001
    spec:
      volumes:
      - name: key
        secret:
          secretName: fluxy-repo-key
      containers:
      - name: fluxy
        image: weaveworks/fluxy:master-a000001
        imagePullPolicy: Never # must build manually
        ports:
        - containerPort: 3030
        volumeMounts:
        - name: key
          mountPath: /var/run/secrets/fluxy/key
          readOnly: true
        args:
        - /home/flux/fluxd
        - --kubernetes-kubectl=/home/flux/kubectl
        - --kubernetes-host=https://kubernetes
        - --kubernetes-certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        - --kubernetes-bearer-token-file=/var/run/secrets/kubernetes.io/serviceaccount/token
        - --database-driver=ql
        - --database-source=file://history.db
        - --repo-url=git@github.com:squaremo/fluxy-testdata
        - --repo-key=/var/run/secrets/fluxy/key/id-rsa
        - --repo-path=testdata
`

const case2image = "weaveworks/fluxy:1234567"

var case2container = []string{"fluxy"}

const case2out = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: fluxy
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: fluxy
        version: "1234567"
    spec:
      volumes:
      - name: key
        secret:
          secretName: fluxy-repo-key
      containers:
      - name: fluxy
        image: weaveworks/fluxy:1234567
        imagePullPolicy: Never # must build manually
        ports:
        - containerPort: 3030
        volumeMounts:
        - name: key
          mountPath: /var/run/secrets/fluxy/key
          readOnly: true
        args:
        - /home/flux/fluxd
        - --kubernetes-kubectl=/home/flux/kubectl
        - --kubernetes-host=https://kubernetes
        - --kubernetes-certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        - --kubernetes-bearer-token-file=/var/run/secrets/kubernetes.io/serviceaccount/token
        - --database-driver=ql
        - --database-source=file://history.db
        - --repo-url=git@github.com:squaremo/fluxy-testdata
        - --repo-key=/var/run/secrets/fluxy/key/id-rsa
        - --repo-path=testdata
`

const case2reverseImage = `weaveworks/fluxy:master-a000001`

const case3 = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
 namespace: monitoring
 name: grafana # comment, and only one space
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: grafana
    spec:
      imagePullSecrets:
      - name: quay-secret
      containers:
      - name: grafana
        image: quay.io/weaveworks/grafana:master-ac5658a
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 80
      - name: gfdatasource
        image: quay.io/weaveworks/gfdatasource:master-e50ecf2
        imagePullPolicy: IfNotPresent
        args:
        - http://prometheus.monitoring.svc.cluster.local/admin/prometheus
`

const case3image = "quay.io/weaveworks/grafana:master-37aaf67"

var case3container = []string{"grafana"}

const case3out = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
 namespace: monitoring
 name: grafana # comment, and only one space
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: grafana
    spec:
      imagePullSecrets:
      - name: quay-secret
      containers:
      - name: grafana
        image: quay.io/weaveworks/grafana:master-37aaf67
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 80
      - name: gfdatasource
        image: quay.io/weaveworks/gfdatasource:master-e50ecf2
        imagePullPolicy: IfNotPresent
        args:
        - http://prometheus.monitoring.svc.cluster.local/admin/prometheus
`

const case4 = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: front-end
  namespace: sock-shop
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: front-end
    spec:
      containers:
      - name: front-end
        image: weaveworksdemos/front-end:v_0.2.0
        resources:
          requests:
            cpu: 100m
            memory: 100Mi
        ports:
        - containerPort: 8079
        securityContext:
          runAsNonRoot: true
          runAsUser: 10001
          capabilities:
            drop:
              - all
          readOnlyRootFilesystem: true
`

const case4image = "weaveworksdemos/front-end:7f511af2d21fd601b86b3bed7baa6adfa9c8c669"

var case4container = []string{"front-end"}

const case4out = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: front-end
  namespace: sock-shop
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: front-end
    spec:
      containers:
      - name: front-end
        image: weaveworksdemos/front-end:7f511af2d21fd601b86b3bed7baa6adfa9c8c669
        resources:
          requests:
            cpu: 100m
            memory: 100Mi
        ports:
        - containerPort: 8079
        securityContext:
          runAsNonRoot: true
          runAsUser: 10001
          capabilities:
            drop:
              - all
          readOnlyRootFilesystem: true
`

const case5 = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
        ports:
        - containerPort: 80
`

const case5image = "nginx:1.10-alpine"

var case5container = []string{"nginx"}

const case5out = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.10-alpine
        ports:
        - containerPort: 80
`

const case6 = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: nginx
    spec:
      containers:
      - ports:
        - containerPort: 80
        image: nginx
        name: nginx
      - image: nginx:some-other-tag # testing comments, and this image is on the first line.
        name: nginx2
`

const case6image = "nginx:1.10-alpine"

var case6containers = []string{"nginx", "nginx2"}

const case6out = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: nginx
    spec:
      containers:
      - ports:
        - containerPort: 80
        image: nginx:1.10-alpine
        name: nginx
      - image: nginx:1.10-alpine # testing comments, and this image is on the first line.
        name: nginx2
`

const case7 = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: authfe
spec:

  # A couple of comment lines after a blank line
  # since that's essentially what we have elsewhere
  minReadySeconds: 30
  strategy:
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1

  replicas: 5
  template:
    metadata:
      labels:
        name: authfe
      annotations:
        prometheus.io.port: "8080"
    spec:
      # blank comment spacers in the following
      containers:
      - name: authfe
        image: quay.io/weaveworks/authfe:master-71a4ede
        args:
        - -log.level=info
        #
        # Can have a comment here
        - -log.blargle=false
        #
        ports:
        - containerPort: 80
          name: http
        - containerPort: 8080
          name: private
      #
      - name: logging
        image: quay.io/weaveworks/logging:master-ccfa2af
        env:
        - name: FLUENTD_CONF
          value: fluent.conf
`

const case7image = "quay.io/weaveworks/logging:master-123456"

var case7containers = []string{"logging"}

const case7out = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: authfe
spec:

  # A couple of comment lines after a blank line
  # since that's essentially what we have elsewhere
  minReadySeconds: 30
  strategy:
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1

  replicas: 5
  template:
    metadata:
      labels:
        name: authfe
      annotations:
        prometheus.io.port: "8080"
    spec:
      # blank comment spacers in the following
      containers:
      - name: authfe
        image: quay.io/weaveworks/authfe:master-71a4ede
        args:
        - -log.level=info
        #
        # Can have a comment here
        - -log.blargle=false
        #
        ports:
        - containerPort: 80
          name: http
        - containerPort: 8080
          name: private
      #
      - name: logging
        image: quay.io/weaveworks/logging:master-123456
        env:
        - name: FLUENTD_CONF
          value: fluent.conf
`

const listImageA = "quay.io/weaveworks/my-deployment:master-456"

var listImageContainer = []string{"my-deployment"}

const singleList = `---
apiVersion: v1
kind: List
items:
  - apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: my-deployment
      labels:
        name: my-deployment
    spec:
      replicas: 1
      template:
        metadata:
          labels:
            name: my-deployment
        spec:
          containers:
            - name: my-deployment
              args:
                - '-m 64'
                - '-p 11211'
              image: quay.io/weaveworks/my-deployment:master-123
              imagePullPolicy: IfNotPresent
              ports:
                - name: clients
                  containerPort: 11211
`

const singleListOut = `---
apiVersion: v1
kind: List
items:
  - apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: my-deployment
      labels:
        name: my-deployment
    spec:
      replicas: 1
      template:
        metadata:
          labels:
            name: my-deployment
        spec:
          containers:
            - name: my-deployment
              args:
                - '-m 64'
                - '-p 11211'
              image: quay.io/weaveworks/my-deployment:master-456
              imagePullPolicy: IfNotPresent
              ports:
                - name: clients
                  containerPort: 11211
`
