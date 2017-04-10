package testdata

import (
	"io/ioutil"
	"os"
	"os/exec"
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
			if err = exec.Command("rm", "-rf", newDir).Run(); err == nil {
				println("Deleted " + newDir)
			} else {
				println("Failed to delete " + newDir)
			}
		} else {
			println("Refusing to delete " + newDir)
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
		flux.ServiceID("default/helloworld"): []string{filepath.Join(dir, "helloworld-deploy.yaml")},
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
      - name: helloworld
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
}
