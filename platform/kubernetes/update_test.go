package kubernetes

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func testUpdate(t *testing.T, name, caseIn, updatedImage, caseOut string) {
	var trace, out bytes.Buffer
	if err := tryUpdate(caseIn, updatedImage, &trace, &out); err != nil {
		fmt.Fprintln(os.Stderr, "Failed:", name)
		fmt.Fprintf(os.Stderr, "--- TRACE ---\n"+trace.String()+"\n---\n")
		t.Fatal(err)
	}
	if string(out.Bytes()) != caseOut {
		fmt.Fprintln(os.Stderr, "Failed:", name)
		fmt.Fprintf(os.Stderr, "--- TRACE ---\n"+trace.String()+"\n---\n")
		t.Fatalf("Did not get expected result, instead got\n\n%s", string(out.Bytes()))
	}
}

func TestUpdates(t *testing.T) {
	for _, c := range [][]string{
		{"common case", case1, case1image, case1out},
		{"new version like number", case2, case2image, case2out},
		{"old version like number", case2out, case2reverseImage, case2},
		{"name label out of order", case3, case3image, case3out},
	} {
		testUpdate(t, c[0], c[1], c[2], c[3])
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

const case1image = `quay.io/weaveworks/pr-assigner:master-1234567`

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

const case2image = `weaveworks/fluxy:1234567`

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
  name: grafana
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

const case3image = `quay.io/weaveworks/grafana:master-37aaf67`

const case3out = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  namespace: monitoring
  name: grafana
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
