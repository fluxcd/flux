package kubernetes

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func TestTryUpdateCase1(t *testing.T) {
	var trace bytes.Buffer
	if err := tryUpdate(case1, "quay.io/weaveworks/pr-assigner:master-1234567", &trace, ioutil.Discard); err != nil {
		fmt.Fprintf(os.Stderr, "--- TRACE ---\n"+trace.String()+"\n---\n")
		t.Error(err)
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
