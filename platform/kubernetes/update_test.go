package kubernetes

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestTryUpdateCase1(t *testing.T) {
	var trace, out bytes.Buffer
	if err := tryUpdate(case1, "quay.io/weaveworks/pr-assigner:master-1234567", &trace, &out); err != nil {
		fmt.Fprintf(os.Stderr, "--- TRACE ---\n"+trace.String()+"\n---\n")
		t.Error(err)
	}
	if string(out.Bytes()) != case1out {
		t.Errorf("Did not get expected result:\n%s", string(out.Bytes()))
	}
}

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

const case1out = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: "pr-assigner"
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
          image: "quay.io/weaveworks/pr-assigner:master-1234567"
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
